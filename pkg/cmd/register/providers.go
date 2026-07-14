package register

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"

	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

// LinuxPlatform reports compatibility based on whether the OS is Linux.
type LinuxPlatform struct{}

func (LinuxPlatform) IsCompatible() bool { return runtime.GOOS == "linux" }

// TerminalPrompter wraps terminal.PromptSelectInput for interactive prompts.
type TerminalPrompter struct{}

func (TerminalPrompter) ConfirmYesNo(label string) bool {
	result := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label: label,
		Items: []string{"Yes, proceed", "No, cancel"},
	})
	return result == "Yes, proceed"
}

func (TerminalPrompter) Select(label string, items []string) string {
	return terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label: label,
		Items: items,
	})
}

// Netbird handles NetBird installation and uninstallation.
type Netbird struct{}

func (Netbird) Install() error   { return InstallNetbird() }
func (Netbird) Uninstall() error { return UninstallNetbird() }

// EnsureRunning checks if the netbird systemd service is active and attempts
// to start it if it is not. It also checks the netbird peer connection status
// and runs "netbird up" if the peer is disconnected.
func (Netbird) EnsureRunning() error {
	out, err := exec.Command("systemctl", "is-active", "netbird").Output() //nolint:gosec // fixed service name
	if err != nil || strings.TrimSpace(string(out)) != "active" {
		if startErr := exec.Command("sudo", "systemctl", "start", "netbird").Run(); startErr != nil { //nolint:gosec // fixed service name
			return fmt.Errorf("failed to start Brev tunnel service: %w", startErr)
		}
	}

	statusOut, err := exec.Command("netbird", "status").Output() //nolint:gosec // fixed command
	if err != nil {
		// Service is running, just can't confirm peer status.
		return nil
	}

	if netbirdManagementConnected(string(statusOut)) {
		return nil
	}

	if upErr := exec.Command("sudo", "netbird", "up").Run(); upErr != nil { //nolint:gosec // fixed command
		return fmt.Errorf("failed to reconnect Brev tunnel: %w", upErr)
	}
	return nil
}

// ShellSetupRunner runs setup scripts via shell.
type ShellSetupRunner struct{}

func (ShellSetupRunner) RunSetup(script string) error { return runSetupCommand(script) }

// DefaultNodeClientFactory creates real ConnectRPC clients.
type DefaultNodeClientFactory struct{}

func (DefaultNodeClientFactory) NewNodeClient(provider externalnode.TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient {
	return NewNodeServiceClient(provider, baseURL)
}
