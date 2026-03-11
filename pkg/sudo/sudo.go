package sudo

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/brevdev/brev-cli/pkg/terminal"
)

// Status represents the current sudo/root status of the process.
type Status int

const (
	// StatusRoot means the process is already running as root.
	StatusRoot Status = 0
	// StatusCached means the user has a valid sudo timestamp (no password needed).
	StatusCached Status = 1
	// StatusUncached means sudo will require a password prompt.
	StatusUncached Status = 2
)

// Gater gates execution on sudo availability; callers can use Default or inject a stub in tests.
type Gater interface {
	Gate(t *terminal.Terminal, confirmer terminal.Confirmer, reason string, assumeYes bool) error
}

// Default is the Gater used by commands. Tests may replace it with a stub.
var Default Gater = &systemGater{}

// systemGater implements Gater using the real sudo check and optional password prompt.
type systemGater struct{}

func (g *systemGater) Gate(t *terminal.Terminal, confirmer terminal.Confirmer, reason string, assumeYes bool) error {
	status := check()
	if status == StatusRoot {
		return nil
	}

	t.Vprint("")
	t.Vprintf("  %s requires elevated (sudo) privileges.\n", reason)
	if status == StatusCached {
		t.Vprint("  Your sudo credentials are cached — no password will be needed.")
	} else {
		t.Vprint("  You will be prompted for your password by sudo.")
	}
	t.Vprint("")

	if !assumeYes && !confirmer.ConfirmYesNo(fmt.Sprintf("%s requires sudo. Continue?", reason)) {
		return fmt.Errorf("%s canceled by user", reason)
	}

	if status == StatusUncached {
		if exec.Command("sudo", "-n", "-v").Run() != nil { //nolint:gosec // intentional sudo -n -v
			if isTTY(os.Stdin) {
				cmd := exec.Command("sudo", "-v") //nolint:gosec // intentional sudo -v
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("sudo authentication failed: %w", err)
				}
			}
		}
	}

	return nil
}

// check returns the current sudo status.
func check() Status {
	if os.Getuid() == 0 {
		return StatusRoot
	}
	if exec.Command("sudo", "-n", "true").Run() == nil { //nolint:gosec // intentional sudo probe
		return StatusCached
	}
	return StatusUncached
}

func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}


// CachedGater is a Gater that always acts as if sudo is cached (no prompt, no refresh). For tests.
type CachedGater struct{}

func (CachedGater) Gate(*terminal.Terminal, terminal.Confirmer, string, bool) error { return nil }

