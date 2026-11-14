package register

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/shlex"
)

// InstallNetbird installs NetBird if it is not already present.
func InstallNetbird() error {
	if _, err := exec.LookPath("netbird"); err == nil {
		return nil
	}

	script := `(curl -fsSL https://pkgs.netbird.io/install.sh | sh) || (curl -fsSL https://pkgs.netbird.io/install.sh | sh -s -- --update)`

	cmd := exec.Command("sudo", "bash", "-c", script) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Brev tunnel: %w", err)
	}
	return nil
}

// runSetupCommand executes the setup command returned by the AddNode RPC.
// Only supported registration arguments are passed to NetBird, without a shell.
func runSetupCommand(script string) error {
	cmd, err := setupCommand(script)
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setup command failed: %w", err)
	}
	return nil
}

func setupCommand(script string) (*exec.Cmd, error) {
	args, err := shlex.Split(script)
	if err != nil || len(args) < 2 || args[0] != "netbird" || args[1] != "up" {
		return nil, fmt.Errorf("unexpected setup command")
	}
	for i := 2; i < len(args); i++ {
		flag, value, hasValue := strings.Cut(args[i], "=")
		switch flag {
		case "--setup-key", "--hostname", "--management-url":
		default:
			return nil, fmt.Errorf("unexpected setup command")
		}
		if !hasValue {
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("unexpected setup command")
			}
			value = args[i]
		}
		if value == "" || strings.HasPrefix(value, "-") {
			return nil, fmt.Errorf("unexpected setup command")
		}
	}
	return exec.Command("sudo", append([]string{"netbird", "up"}, args[2:]...)...), nil // #nosec G204 -- fixed executable and allowlisted arguments; no shell
}

// UninstallNetbird stops, uninstalls the service, and removes the NetBird
// package or binary. It reads /etc/netbird/install.conf (written by the
// install script) to determine the original installation method.
// The down/stop steps are best-effort since the service may already be
// disconnected or stopped after deregistration.
func UninstallNetbird() error {
	script := `
netbird down 2>/dev/null || true
netbird service stop 2>/dev/null || true
netbird service uninstall 2>/dev/null || true

PKG_MGR="bin"
if [ -f /etc/netbird/install.conf ]; then
  PKG_MGR=$(grep -oP '(?<=package_manager=)\S+' /etc/netbird/install.conf 2>/dev/null || echo "bin")
fi

case "$PKG_MGR" in
  apt)  apt-get remove -y netbird || true ;;
  dnf)  dnf remove -y netbird || true ;;
  yum)  yum remove -y netbird || true ;;
  *)    rm -f /usr/bin/netbird /usr/local/bin/netbird ;;
esac

rm -rf /etc/netbird
rm -rf /var/lib/netbird
rm -f /usr/bin/netbird /usr/local/bin/netbird
`

	cmd := exec.Command("sudo", "bash", "-c", script) // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall Brev tunnel: %w", err)
	}
	return nil
}
