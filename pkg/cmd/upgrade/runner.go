package upgrade

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/brevdev/brev-cli/pkg/terminal"
)

const installScriptURL = "https://raw.githubusercontent.com/brevdev/brev-cli/main/bin/install-latest.sh"

// Upgrader executes the actual upgrade process.
type Upgrader interface {
	UpgradeViaBrew(t *terminal.Terminal) error
	UpgradeViaInstallScript(t *terminal.Terminal) error
}

// SystemUpgrader executes upgrade commands on the real system.
type SystemUpgrader struct{}

// UpgradeViaBrew runs "brew upgrade brev" with output connected to the terminal.
func (SystemUpgrader) UpgradeViaBrew(t *terminal.Terminal) error {
	t.Vprint("Running: brew upgrade brev")
	t.Vprint("")

	cmd := exec.Command("brew", "upgrade", "brev")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew upgrade failed: %w", err)
	}
	return nil
}

// UpgradeViaInstallScript runs the upstream install-latest.sh script via sudo.
func (SystemUpgrader) UpgradeViaInstallScript(t *terminal.Terminal) error {
	t.Vprintf("Running: bash -c \"$(curl -fsSL %s)\"\n", installScriptURL)
	t.Vprint("")

	cmd := exec.Command("bash", "-c", fmt.Sprintf(`curl -fsSL "%s" | bash`, installScriptURL)) //nolint:gosec // URL is a compile-time constant
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install script failed: %w", err)
	}
	return nil
}
