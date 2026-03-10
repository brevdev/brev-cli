// Package sudo provides utilities for checking and gating on sudo access.
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

// Check returns the current sudo status.
func Check() Status {
	if os.Getuid() == 0 {
		return StatusRoot
	}
	if exec.Command("sudo", "-n", "true").Run() == nil { //nolint:gosec // intentional sudo probe
		return StatusCached
	}
	return StatusUncached
}

// Gate checks sudo status, informs the user, and asks for confirmation before
// proceeding. Returns nil if the user confirms (or is already root). Returns
// an error if the user declines. When assumeYes is true, the confirmation prompt is skipped.
func Gate(t *terminal.Terminal, confirmer terminal.Confirmer, reason string, assumeYes bool) error {
	status := Check()
	if status == StatusRoot {
		return nil
	}

	if assumeYes {
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

	if !confirmer.ConfirmYesNo(fmt.Sprintf("%s requires sudo. Continue?", reason)) {
		return fmt.Errorf("%s canceled by user", reason)
	}

	return nil
}
