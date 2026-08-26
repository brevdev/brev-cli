// Package deregister provides the brev deregister command for device deregistration
package deregister

import (
	"context"
	"errors"
	"fmt"
	"os/user"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/sshcert"
	"github.com/brevdev/brev-cli/pkg/sudo"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type DeregisterStore interface {
	GetCurrentUser() (*entity.User, error)
	GetAccessToken() (string, error)
}

type CertAuthorityRemover interface {
	RemoveCertAuthority(u *user.User, nodeID, linuxUser string) (bool, error)
}

// LegacySSHKeyRemover removes Brev-managed per-user SSH keys (legacy nodes).
type LegacySSHKeyRemover interface {
	RemoveBrevKeys(u *user.User) ([]string, error)
}

type brevCertAuthorityRemover struct{}

func (brevCertAuthorityRemover) RemoveCertAuthority(u *user.User, nodeID, linuxUser string) (bool, error) {
	removed, err := sshcert.RemoveCertAuthorityLine(u.HomeDir, nodeID, linuxUser)
	return removed, breverrors.WrapAndTrace(err)
}

type legacyKeyRemover struct{}

func (legacyKeyRemover) RemoveBrevKeys(u *user.User) ([]string, error) {
	removed, err := register.RemoveBrevAuthorizedKeys(u)
	return removed, breverrors.WrapAndTrace(err)
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
	sshKeys           CertAuthorityRemover
	legacyKeys        LegacySSHKeyRemover
	// currentUser resolves the OS user for authorized_keys operations.
	currentUser func() (*user.User, error)
}

func defaultDeregisterDeps() deregisterDeps {
	return deregisterDeps{
		platform:          register.LinuxPlatform{},
		prompter:          register.TerminalPrompter{},
		confirmer:         register.TerminalPrompter{},
		gater:             sudo.Default,
		netbird:           register.Netbird{},
		nodeClients:       register.DefaultNodeClientFactory{},
		sshKeys:           brevCertAuthorityRemover{},
		legacyKeys:        legacyKeyRemover{},
		registrationStore: register.NewFileRegistrationStore(),
		currentUser:       user.Current,
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

func removeNodeFromBrev(ctx context.Context, t *terminal.Terminal, s DeregisterStore, deps deregisterDeps, reg *register.DeviceRegistration) error {
	externalNodeID := reg.ExternalNodeID
	if externalNodeID == "" && reg.DeviceID != "" {
		lookedUp, lookupErr := findNodeByDeviceID(ctx, s, deps, reg.OrgID, reg.DeviceID)
		if lookupErr != nil {
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Failed look up by device ID. Please try again: %v", lookupErr)))
			return fmt.Errorf("failed to find pending node by device ID")
		}
		if lookedUp != "" {
			externalNodeID = lookedUp
		}
	}
	if externalNodeID == "" {
		t.Vprintf("  %s\n", t.Yellow("No registered node to remove (pending registration); cleaning up local state."))
		return nil
	}
	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	_, err := client.RemoveNode(ctx, connect.NewRequest(&nodev1.RemoveNodeRequest{
		ExternalNodeId: externalNodeID,
	}))
	if err != nil {
		var connectErr *connect.Error
		if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeNotFound {
			t.Vprintf("  %s\n", t.Yellow("Node not found on Brev; continuing."))
			return nil
		}
		return fmt.Errorf("failed to deregister node: %w", err)
	}
	t.Vprintf("%s  Node removed from Brev.\n", t.Green("  ✓"))
	return nil
}

func findNodeByDeviceID(ctx context.Context, s externalnode.TokenProvider, deps deregisterDeps, orgID, deviceID string) (string, error) {
	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	pageToken := ""
	for {
		resp, err := client.ListNodes(ctx, connect.NewRequest(&nodev1.ListNodesRequest{
			OrganizationId: orgID,
			PageParams: &nodev1.PageParams{
				PageSize:  100,
				PageToken: pageToken,
			},
		}))
		if err != nil {
			return "", fmt.Errorf("failed to list nodes: %w", err)
		}
		for _, n := range resp.Msg.GetItems() {
			if n.GetDeviceId() == deviceID {
				return n.GetExternalNodeId(), nil
			}
		}
		pageToken = resp.Msg.GetNextPageToken()
		if pageToken == "" {
			return "", nil
		}
	}
}

func runDeregister(ctx context.Context, t *terminal.Terminal, s DeregisterStore, deps deregisterDeps, skipConfirm bool) error { //nolint:funlen,gocyclo // deregistration flow
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev deregister is only supported on Linux")
	}

	if err := deps.gater.Gate(t, deps.confirmer, "Device deregistration", skipConfirm); err != nil {
		return fmt.Errorf("sudo issue: %w", err)
	}

	reg, err := deps.registrationStore.LoadAll() // deregister should still work for pending registrations
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
	osUser, _ := deps.currentUser()
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
	t.Vprint("    2. Remove any SSH data associated with this node")
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

	// a Brev cert-authority line for this node means certauth mode, otherwise legacy per-user keys
	certAuth := false
	if osUser != nil {
		certAuth = sshcert.HasCertAuthorityLine(osUser.HomeDir, reg.ExternalNodeID)
	}

	t.Vprint(t.Yellow("[Step 1/4] Removing node from Brev..."))
	if err := removeNodeFromBrev(ctx, t, s, deps, reg); err != nil {
		return err
	}
	t.Vprint("")

	t.Vprint(t.Yellow("[Step 2/4] Removing any SSH data associated with this node..."))
	if osUser == nil {
		t.Vprintf("  %s\n", t.Yellow("Skipped: could not determine current user"))
	} else {
		linuxUsername := osUser.Username
		if certAuth {
			removeCertAuthorityStep(t, deps, osUser, reg.ExternalNodeID, linuxUsername)
		} else {
			removeLegacyKeysStep(t, deps, osUser)
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

func removeCertAuthorityStep(t *terminal.Terminal, deps deregisterDeps, osUser *user.User, nodeID, linuxUser string) {
	removed, cerr := deps.sshKeys.RemoveCertAuthority(osUser, nodeID, linuxUser)
	switch {
	case cerr != nil:
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to remove cert-authority: %v", cerr)))
	case removed:
		t.Vprintf("%s  Certificate authority removed from authorized_keys.\n", t.Green("  ✓"))
	default:
		t.Vprint("  No certificate authority line found in authorized_keys.")
	}
}

func removeLegacyKeysStep(t *terminal.Terminal, deps deregisterDeps, osUser *user.User) {
	removed, kerr := deps.legacyKeys.RemoveBrevKeys(osUser)
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
