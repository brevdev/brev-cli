// Package deregister provides the brev deregister command for device deregistration
package deregister

import (
	"context"
	"fmt"
	"os/user"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// DeregisterStore defines the store methods needed by the deregister command.
type DeregisterStore interface {
	GetCurrentUser() (*entity.User, error)
	GetAccessToken() (string, error)
}

// SSHKeyRemover removes Brev-managed SSH keys and returns the lines removed.
type SSHKeyRemover interface {
	RemoveBrevKeys(u *user.User) ([]string, error)
}

// brevSSHKeyRemover delegates to register.RemoveBrevAuthorizedKeys.
type brevSSHKeyRemover struct{}

func (brevSSHKeyRemover) RemoveBrevKeys(u *user.User) ([]string, error) {
	removed, err := register.RemoveBrevAuthorizedKeys(u)
	if err != nil {
		return nil, fmt.Errorf("removing brev authorized keys: %w", err)
	}
	return removed, nil
}

// deregisterDeps bundles the side-effecting dependencies of runDeregister so
// they can be replaced in tests.
type deregisterDeps struct {
	platform          externalnode.PlatformChecker
	prompter          terminal.Selector
	netbird           register.NetBirdManager
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	sshKeys           SSHKeyRemover
}

func defaultDeregisterDeps() deregisterDeps {
	return deregisterDeps{
		platform:          register.LinuxPlatform{},
		prompter:          register.TerminalPrompter{},
		netbird:           register.Netbird{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
		sshKeys:           brevSSHKeyRemover{},
	}
}

var (
	deregisterLong = `Deregister your device from NVIDIA Brev

This command removes the local registration data and uninstalls
the Brev tunnel (network agent).`

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
			return runDeregister(cmd.Context(), t, store, defaultDeregisterDeps())
		},
	}

	return cmd
}

func runDeregister(ctx context.Context, t *terminal.Terminal, s DeregisterStore, deps deregisterDeps) error { //nolint:funlen // deregistration flow
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev deregister is only supported on Linux")
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return breverrors.WrapAndTrace(err)
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
	osUser, err := user.Current()
	if err != nil {
		t.Vprintf("  Warning: could not determine current user for SSH key cleanup: %v\n", err)
	} else {
		removed, kerr := deps.sshKeys.RemoveBrevKeys(osUser)
		switch {
		case kerr != nil:
			t.Vprintf("  Warning: failed to remove Brev SSH keys: %v\n", kerr)
		case len(removed) > 0:
			t.Vprint(t.Green("  Brev SSH keys removed from authorized_keys:"))
			for _, key := range removed {
				t.Vprintf("    - %s\n", key)
			}
		default:
			t.Vprint("  No Brev SSH keys found in authorized_keys.")
		}
	}
	t.Vprint("")

	t.Vprint("Removing Brev tunnel...")
	if err := deps.netbird.Uninstall(); err != nil {
		t.Vprintf("  Warning: failed to remove Brev tunnel: %v\n", err)
	} else {
		t.Vprint(t.Green("  Brev tunnel removed."))
	}
	t.Vprint("")

	t.Vprint("Removing registration data...")
	if err := deps.registrationStore.Delete(); err != nil {
		t.Vprintf("  Warning: failed to remove local registration file: %v\n", err)
		t.Vprint("  You can manually remove it with: rm /etc/brev/device_registration.json")
	}

	t.Vprint(t.Green("Deregistration complete."))
	t.Vprint("")

	return nil
}
