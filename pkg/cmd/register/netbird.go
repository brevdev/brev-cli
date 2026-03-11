package register

import (
	"fmt"
	"os/exec"
	"strings"
)

// InstallNetbird installs NetBird if it is not already present.
func InstallNetbird() error {
	if _, err := exec.LookPath("netbird"); err == nil {
		return nil
	}

	script := `(curl -fsSL https://pkgs.netbird.io/install.sh | sh) || (curl -fsSL https://pkgs.netbird.io/install.sh | sh -s -- --update)`

	cmd := exec.Command("bash", "-c", script) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Brev tunnel: %w", err)
	}
	return nil
}

// runSetupCommand executes the setup command returned by the AddNode RPC.
// It validates that the command starts with "netbird up" as a basic guard
// against unexpected server responses.
func runSetupCommand(script string) error {
	if !strings.HasPrefix(strings.TrimSpace(script), "netbird up") {
		return fmt.Errorf("unexpected setup command")
	}
	cmd := exec.Command("bash", "-c", script) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setup command failed: %w", err)
	}
	return nil
}

// UninstallNetbird stops, uninstalls the service, and removes the NetBird
// package or binary. It reads /etc/netbird/install.conf (written by the
// install script) to determine the original installation method.
// The down/stop steps are best-effort since the service may already be
// disconnected or stopped after deregistration.
func UninstallNetbird() error {
	script := `
sudo netbird down 2>/dev/null
sudo netbird service stop 2>/dev/null
sudo netbird service uninstall 2>/dev/null

PKG_MGR="bin"
if [ -f /etc/netbird/install.conf ]; then
  PKG_MGR=$(grep -oP '(?<=package_manager=)\S+' /etc/netbird/install.conf 2>/dev/null || echo "bin")
fi

case "$PKG_MGR" in
  apt)  sudo apt-get remove -y netbird || true ;;
  dnf)  sudo dnf remove -y netbird || true ;;
  yum)  sudo yum remove -y netbird || true ;;
  *)    sudo rm -f /usr/bin/netbird /usr/local/bin/netbird ;;
esac

sudo rm -rf /etc/netbird
sudo rm -rf /var/lib/netbird
sudo rm -f /usr/bin/netbird /usr/local/bin/netbird
`

	cmd := exec.Command("bash", "-c", script) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall Brev tunnel: %w", err)
	}
	return nil
}
