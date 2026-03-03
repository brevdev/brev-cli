package register

import (
	"runtime"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"

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

// NetBirdManager handles NetBird installation and uninstallation.
type NetBirdManager struct{}

func (NetBirdManager) Install() error   { return InstallNetbird() }
func (NetBirdManager) Uninstall() error { return UninstallNetbird() }

// ShellSetupRunner runs setup scripts via shell.
type ShellSetupRunner struct{}

func (ShellSetupRunner) RunSetup(script string) error { return runSetupCommand(script) }

// DefaultNodeClientFactory creates real ConnectRPC clients.
type DefaultNodeClientFactory struct{}

func (DefaultNodeClientFactory) NewNodeClient(provider TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient {
	return NewNodeServiceClient(provider, baseURL)
}
