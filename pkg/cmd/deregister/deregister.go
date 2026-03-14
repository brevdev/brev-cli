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
	"github.com/brevdev/brev-cli/pkg/sudo"
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
	confirmer         terminal.Confirmer
	gater             sudo.Gater
	netbird           register.NetBirdManager
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	sshKeys           SSHKeyRemover
}

func defaultDeregisterDeps() deregisterDeps {
	return deregisterDeps{
		platform:          register.LinuxPlatform{},
		prompter:          register.TerminalPrompter{},
		confirmer:         register.TerminalPrompter{},
		gater:             sudo.Default,
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
	var approveFlag bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "deregister",
		DisableFlagsInUseLine: true,
		Short:                 "Deregister your device from Brev",
		Long:                  deregisterLong,
		Example:               deregisterExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeregister(cmd.Context(), t, store, defaultDeregisterDeps(), approveFlag)
		},
	}

	cmd.Flags().BoolVar(&approveFlag, "approve", false, "skip confirmation prompt (assume yes)")

	return cmd
}

func runDeregister(ctx context.Context, t *terminal.Terminal, s DeregisterStore, deps deregisterDeps, skipConfirm bool) error { //nolint:funlen,gocyclo // deregistration flow
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev deregister is only supported on Linux")
	}

	if err := deps.gater.Gate(t, deps.confirmer, "Device deregistration", skipConfirm); err != nil {
		return fmt.Errorf("sudo issue: %w", err)
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return err //nolint:wrapcheck // do not present stack trace for this error
	}

	// Only prompt for login when there is a device to deregister.
	if _, err := s.GetCurrentUser(); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	orgName := reg.OrgName
	if orgName == "" {
		orgName = "(unknown)"
	}
	osUser, _ := user.Current()
	linuxUser := "(unknown)"
	if osUser != nil {
		linuxUser = osUser.Username
	}

	t.Vprint("")
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint(t.White("  Deregistering your device from Brev"))
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint("")
	if !skipConfirm {
		t.Vprint(t.Green("  Please confirm before continuing:"))
		t.Vprint("")
	}
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Device:")), t.BoldBlue(reg.DisplayName+" ("+reg.ExternalNodeID+")"))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Organization:")), t.BoldBlue(orgName+" ("+reg.OrgID+")"))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Linux user:")), t.BoldBlue(linuxUser))
	t.Vprint("")
	t.Vprint(t.Yellow("  This will:"))
	t.Vprint("    1. Remove this node from Brev")
	t.Vprint("    2. Remove Brev SSH keys from this machine (if any)")
	t.Vprint("    3. Uninstall the Brev tunnel")
	t.Vprint("    4. Delete local registration data")
	t.Vprint("")

	if !skipConfirm {
		confirm := deps.prompter.Select(
			"Proceed with deregistration?",
			[]string{"Yes, proceed", "No, cancel"},
		)
		if confirm != "Yes, proceed" {
			t.Vprint("Deregistration canceled.")
			return nil
		}
	}

	t.Vprint(t.Yellow("[Step 1/4] Removing node from Brev..."))
	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	_, err = client.RemoveNode(ctx, connect.NewRequest(&nodev1.RemoveNodeRequest{
		ExternalNodeId: reg.ExternalNodeID,
	}))
	if err != nil {
		return fmt.Errorf("failed to deregister node: %w", err)
	}
	t.Vprintf("%s  Node removed from Brev.\n", t.Green("  ✓"))
	t.Vprint("")

	t.Vprint(t.Yellow("[Step 2/4] Removing Brev SSH keys..."))
	if osUser == nil {
		t.Vprintf("  %s\n", t.Yellow("Skipped: could not determine current user"))
	} else {
		removed, kerr := deps.sshKeys.RemoveBrevKeys(osUser)
		switch {
		case kerr != nil:
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to remove Brev SSH keys: %v", kerr)))
		case len(removed) > 0:
			t.Vprintf("%s  Brev SSH keys removed from authorized_keys:\n", t.Green("  ✓"))
			for _, key := range removed {
				t.Vprintf("    - %s\n", key)
			}
		default:
			t.Vprint("  No Brev SSH keys found in authorized_keys.")
		}
	}
	t.Vprint("")

	t.Vprint(t.Yellow("[Step 3/4] Removing Brev tunnel..."))
	err = deps.netbird.Uninstall()
	if err != nil {
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to remove Brev tunnel: %v", err)))
	} else {
		t.Vprintf("%s  Brev tunnel removed.\n", t.Green("  ✓"))
	}
	t.Vprint("")

	t.Vprint(t.Yellow("[Step 4/4] Removing registration data..."))
	err = deps.registrationStore.Delete()
	if err != nil {
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to remove local registration file: %v", err)))
		t.Vprint("  You can manually remove it with: rm /etc/brev/device_registration.json")
	} else {
		t.Vprintf("%s  Registration data removed.\n", t.Green("  ✓"))
	}
	t.Vprintf("%s  Deregistration complete.\n", t.Green("  ✓"))
	t.Vprint("")

	return nil
}
