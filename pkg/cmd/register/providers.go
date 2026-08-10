package register

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

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

const (
	defaultNetBirdConnectTimeout = 30 * time.Second
	defaultNetBirdPollInterval   = 500 * time.Millisecond
)

type netBirdCommandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
	Run(context.Context, string, ...string) error
}

type execNetBirdCommandRunner struct{}

func (execNetBirdCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (execNetBirdCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Netbird handles NetBird installation and connectivity.
type Netbird struct {
	runner         netBirdCommandRunner
	connectTimeout time.Duration
	pollInterval   time.Duration
}

func (Netbird) Install() error   { return InstallNetbird() }
func (Netbird) Uninstall() error { return UninstallNetbird() }

func (n Netbird) commandRunner() netBirdCommandRunner {
	if n.runner != nil {
		return n.runner
	}
	return execNetBirdCommandRunner{}
}

func (n Netbird) connectionTimeout() time.Duration {
	if n.connectTimeout > 0 {
		return n.connectTimeout
	}
	return defaultNetBirdConnectTimeout
}

func (n Netbird) connectionPollInterval() time.Duration {
	if n.pollInterval > 0 {
		return n.pollInterval
	}
	return defaultNetBirdPollInterval
}

// EnsureConnected ensures the local service is active and confirms that its
// management connection is established before returning.
func (n Netbird) EnsureConnected(ctx context.Context) error {
	runner := n.commandRunner()
	out, err := runner.Output(ctx, "systemctl", "is-active", "netbird")
	if err != nil || strings.TrimSpace(string(out)) != "active" {
		if startErr := runner.Run(ctx, "sudo", "systemctl", "start", "netbird"); startErr != nil {
			return fmt.Errorf("failed to start Brev tunnel service: %w", startErr)
		}
	}

	statusOut, statusErr := runner.Output(ctx, "netbird", "status")
	if statusErr == nil && netbirdManagementConnected(string(statusOut)) {
		return nil
	}

	if upErr := runner.Run(ctx, "sudo", "netbird", "up"); upErr != nil {
		return fmt.Errorf("failed to reconnect Brev tunnel: %w", upErr)
	}

	confirmationCtx, cancel := context.WithTimeout(ctx, n.connectionTimeout())
	defer cancel()
	lastStatusErr := statusErr
	checkStatus := func() bool {
		statusOut, err := runner.Output(confirmationCtx, "netbird", "status")
		if err != nil {
			lastStatusErr = err
			return false
		}
		return netbirdManagementConnected(string(statusOut))
	}

	if checkStatus() {
		return nil
	}

	ticker := time.NewTicker(n.connectionPollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-confirmationCtx.Done():
			if lastStatusErr != nil {
				return fmt.Errorf("Brev tunnel connection was not confirmed: %w", lastStatusErr)
			}
			return fmt.Errorf("Brev tunnel connection was not confirmed")
		case <-ticker.C:
			if checkStatus() {
				return nil
			}
		}
	}
}

// netbirdManagementConnected parses "netbird status" output and returns true
// when the Management line reports "Connected".
func netbirdManagementConnected(statusOutput string) bool {
	for _, line := range strings.Split(statusOutput, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Management:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Management:")) == "Connected"
		}
	}
	return false
}

// ShellSetupRunner runs setup scripts via shell.
type ShellSetupRunner struct{}

func (ShellSetupRunner) RunSetup(script string) error { return runSetupCommand(script) }

// DefaultNodeClientFactory creates real ConnectRPC clients.
type DefaultNodeClientFactory struct{}

func (DefaultNodeClientFactory) NewNodeClient(provider externalnode.TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient {
	return NewNodeServiceClient(provider, baseURL)
}
