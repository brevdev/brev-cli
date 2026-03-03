// Package deregister provides the brev deregister command for device deregistration
package deregister

import (
	"context"
	"fmt"
	"os/user"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// DeregisterStore defines the store methods needed by the deregister command.
type DeregisterStore interface {
	GetCurrentUser() (*entity.User, error)
	GetBrevHomePath() (string, error)
	GetAccessToken() (string, error)
}

// PlatformChecker checks whether the current platform is supported.
type PlatformChecker interface {
	IsCompatible() bool
}

// Selector prompts the user to choose from a list of items.
type Selector interface {
	Select(label string, items []string) string
}

// NetBirdUninstaller uninstalls the NetBird network agent.
type NetBirdUninstaller interface {
	Uninstall() error
}

// NodeClientFactory creates ConnectRPC ExternalNodeService clients.
type NodeClientFactory interface {
	NewNodeClient(provider register.TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient
}

// SSHKeyRemover removes Brev-managed SSH keys.
type SSHKeyRemover interface {
	RemoveBrevKeys(u *user.User) error
}

// brevSSHKeyRemover delegates to register.RemoveBrevAuthorizedKeys.
type brevSSHKeyRemover struct{}

func (brevSSHKeyRemover) RemoveBrevKeys(u *user.User) error {
	if err := register.RemoveBrevAuthorizedKeys(u); err != nil {
		return fmt.Errorf("removing brev authorized keys: %w", err)
	}
	return nil
}

// deregisterDeps bundles the side-effecting dependencies of runDeregister so
// they can be replaced in tests.
type deregisterDeps struct {
	platform          PlatformChecker
	prompter          Selector
	netbird           NetBirdUninstaller
	nodeClients       NodeClientFactory
	registrationStore register.RegistrationStore
	sshKeys           SSHKeyRemover
}

func defaultDeregisterDeps(brevHome string) deregisterDeps {
	return deregisterDeps{
		platform:          register.LinuxPlatform{},
		prompter:          register.TerminalPrompter{},
		netbird:           register.NetBirdManager{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(brevHome),
		sshKeys:           brevSSHKeyRemover{},
	}
}

var (
	deregisterLong = `Deregister your device from NVIDIA Brev

This command removes the local registration data and optionally uninstalls
NetBird (network agent).`

	deregisterExample = `  brev deregister`
)

func NewCmdDeregister(t *terminal.Terminal, store DeregisterStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "deregister",
		DisableFlagsInUseLine: true,
		Short:                 "Deregister your device from Brev",
		Long:                  deregisterLong,
		Example:               deregisterExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			brevHome, err := store.GetBrevHomePath()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return runDeregister(cmd.Context(), t, store, defaultDeregisterDeps(brevHome))
		},
	}

	return cmd
}

func runDeregister(ctx context.Context, t *terminal.Terminal, s DeregisterStore, deps deregisterDeps) error { //nolint:funlen // deregistration flow
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev deregister is only supported on Linux")
	}

	registered, err := deps.registrationStore.Exists()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !registered {
		return fmt.Errorf("no registration found; this machine does not appear to be registered\nRun 'brev register' to register your device")
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return fmt.Errorf("failed to read registration file: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green("Deregistering device"))
	t.Vprint("")
	t.Vprintf("  Node ID: %s\n", reg.ExternalNodeID)
	t.Vprintf("  Name:    %s\n", reg.DisplayName)
	t.Vprint("")

	confirm := deps.prompter.Select(
		"Proceed with deregistration?",
		[]string{"Yes, proceed", "No, cancel"},
	)
	if confirm != "Yes, proceed" {
		t.Vprint("Deregistration canceled.")
		return nil
	}

	t.Vprint("")
	t.Vprint(t.Yellow("Removing node from Brev..."))
	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	if _, err := client.RemoveNode(ctx, connect.NewRequest(&nodev1.RemoveNodeRequest{
		ExternalNodeId: reg.ExternalNodeID,
	})); err != nil {
		return fmt.Errorf("failed to deregister node: %w", err)
	}
	t.Vprint(t.Green("  Node removed from Brev."))
	t.Vprint("")

	// Remove Brev SSH keys from authorized_keys.
	u, err := user.Current()
	if err != nil {
		t.Vprintf("  Warning: could not determine current user for SSH key cleanup: %v\n", err)
	} else {
		if err := deps.sshKeys.RemoveBrevKeys(u); err != nil {
			t.Vprintf("  Warning: failed to remove Brev SSH keys: %v\n", err)
		} else {
			t.Vprint(t.Green("  Brev SSH keys removed from authorized_keys."))
		}
	}
	t.Vprint("")

	removeNetbird := deps.prompter.Select(
		"Would you also like to uninstall NetBird?",
		[]string{"Yes, uninstall NetBird", "No, keep NetBird installed"},
	)
	if removeNetbird == "Yes, uninstall NetBird" {
		t.Vprint("Removing NetBird...")
		if err := deps.netbird.Uninstall(); err != nil {
			t.Vprintf("  Warning: failed to uninstall NetBird: %v\n", err)
		} else {
			t.Vprint(t.Green("  NetBird uninstalled."))
		}
		t.Vprint("")
	}

	t.Vprint("Removing registration data...")
	if err := deps.registrationStore.Delete(); err != nil {
		t.Vprintf("  Warning: failed to remove local registration file: %v\n", err)
		t.Vprint("  You can manually remove it with: rm ~/.brev/device_registration.json")
	}

	t.Vprint(t.Green("Deregistration complete."))
	t.Vprint("")

	return nil
}
