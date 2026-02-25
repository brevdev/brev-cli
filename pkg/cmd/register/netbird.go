package register

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/brevdev/brev-cli/pkg/terminal"
)

// InstallNetbird downloads and installs netbird using the official install script.
func InstallNetbird(t *terminal.Terminal) error {
	script := `(curl -fsSL https://pkgs.netbird.io/install.sh | sh) || (curl -fsSL https://pkgs.netbird.io/install.sh | sh -s -- --update)`

	cmd := exec.Command("bash", "-c", script) // #nosec G204
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install netbird: %w", err)
	}
	return nil
}

// UninstallNetbird stops, uninstalls, and removes netbird.
func UninstallNetbird(t *terminal.Terminal) error {
	script := `netbird service stop && netbird service uninstall && sudo apt-get remove -y netbird`

	cmd := exec.Command("bash", "-c", script) // #nosec G204
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall netbird: %w", err)
	}
	return nil
}
