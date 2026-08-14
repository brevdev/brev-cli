// Package deregister provides the canonical Brev network leave command and
// its deprecated deregister alias.
package deregister

import (
	"context"
	"fmt"
	"io"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/sudo"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// LeaveStore defines the authenticated store methods needed by leave.
type LeaveStore interface {
	GetCurrentUser() (*entity.User, error)
	GetAccessToken() (string, error)
}

// DeregisterStore is retained for source compatibility.
//
// Deprecated: use LeaveStore.
type DeregisterStore = LeaveStore

type netBirdUninstaller interface {
	Uninstall() error
}

type leaveDeps struct {
	platform          externalnode.PlatformChecker
	confirmer         terminal.Confirmer
	gater             sudo.Gater
	netbird           netBirdUninstaller
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
}

func defaultLeaveDeps() leaveDeps {
	return leaveDeps{
		platform:          register.LinuxPlatform{},
		confirmer:         register.TerminalPrompter{},
		gater:             sudo.Default,
		netbird:           register.Netbird{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
	}
}

const leaveLong = `Leave the Brev network

This removes the backend node, uninstalls the Brev tunnel, and deletes local
registration data. It does not revoke SSH access grants; run "brev disable-ssh"
first when those grants should be revoked.`

// NewCmdLeave creates the canonical network-membership teardown command.
func NewCmdLeave(t *terminal.Terminal, store LeaveStore) *cobra.Command {
	return newCmdLeave(t, store, defaultLeaveDeps())
}

// NewCmdDeregister is retained for source compatibility. It returns the
// canonical leave command with deregister as its deprecated alias.
//
// Deprecated: use NewCmdLeave.
func NewCmdDeregister(t *terminal.Terminal, store DeregisterStore) *cobra.Command {
	return NewCmdLeave(t, store)
}

func newCmdLeave(t *terminal.Terminal, store LeaveStore, deps leaveDeps) *cobra.Command {
	var approveFlag bool
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "leave",
		Aliases:               []string{"deregister"},
		DisableFlagsInUseLine: true,
		Short:                 "Leave the Brev network",
		Long:                  leaveLong,
		Example:               "  brev leave\n  brev leave --approve",
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.CalledAs() == "deregister" {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), `Warning: "brev deregister" is deprecated; use "brev leave" instead.`)
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), `This command does not revoke SSH access grants; run "brev disable-ssh" before leaving if you want to revoke them.`)
			}
			return runLeave(cmd.Context(), t, cmd.ErrOrStderr(), store, deps, approveFlag)
		},
	}
	cmd.Flags().BoolVar(&approveFlag, "approve", false, "skip confirmation prompt (assume yes)")
	return cmd
}

func runLeave(
	ctx context.Context,
	t *terminal.Terminal,
	warnings io.Writer,
	store LeaveStore,
	deps leaveDeps,
	skipConfirm bool,
) error { //nolint:funlen // The retry-safe teardown order is intentionally explicit.
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev leave is only supported on Linux")
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return fmt.Errorf("read joined-device registration: %w", err)
	}
	if _, err := store.GetCurrentUser(); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	client := deps.nodeClients.NewNodeClient(store, config.GlobalConfig.GetBrevPublicAPIURL())
	node, missing, err := lookupJoinedNodeForLeave(ctx, client, reg)
	if err != nil {
		return fmt.Errorf("inspect joined node before leaving: %w", err)
	}
	if warnings == nil {
		warnings = io.Discard
	}
	_, _ = fmt.Fprintln(warnings, "Leaving removes the Brev tunnel and may interrupt commands using Brev SSH. Run this locally or through out-of-band access.")
	if missing {
		_, _ = fmt.Fprintln(warnings, "Warning: the backend node is already absent; skipping SSH grant inspection.")
	} else {
		grantCount, accountCount := remainingSSHAccessCounts(node.GetSshAccess())
		if grantCount > 0 {
			_, _ = fmt.Fprintf(warnings, "Warning: %d SSH grants across %d Linux accounts remain on this node.\n", grantCount, accountCount)
			_, _ = fmt.Fprintln(warnings, `Leaving stops Brev-routed SSH but does not revoke these grants. Cancel and run "brev disable-ssh" first if you want them revoked.`)
		}
	}

	t.Vprint("")
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint(t.White("  Leaving the Brev network"))
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprintf("  Node:         %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Organization: %s (%s)\n", reg.OrgName, reg.OrgID)
	t.Vprint("")

	if !skipConfirm && !deps.confirmer.ConfirmYesNo("Leave the Brev network?") {
		t.Vprint("Leave canceled.")
		return nil
	}
	if err := deps.gater.Gate(t, deps.confirmer, "Leave Brev network", true); err != nil {
		return fmt.Errorf("sudo issue: %w", err)
	}

	_, err = client.RemoveNode(ctx, connect.NewRequest(&nodev1.RemoveNodeRequest{
		ExternalNodeId: reg.ExternalNodeID,
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		return fmt.Errorf("leave Brev network: remove node: %w", err)
	}
	if err := deps.netbird.Uninstall(); err != nil {
		return fmt.Errorf("leave Brev network: uninstall tunnel: %w", err)
	}
	if err := deps.registrationStore.Delete(); err != nil {
		return fmt.Errorf("leave Brev network: delete local registration: %w", err)
	}
	t.Vprint("Left the Brev network.")
	return nil
}

func lookupJoinedNodeForLeave(
	ctx context.Context,
	client nodev1connect.ExternalNodeServiceClient,
	reg *register.DeviceRegistration,
) (*nodev1.ExternalNode, bool, error) {
	resp, err := client.ListNodes(ctx, connect.NewRequest(&nodev1.ListNodesRequest{
		OrganizationId: reg.OrgID,
	}))
	if err != nil {
		return nil, false, fmt.Errorf("list organization nodes: %w", err)
	}
	if resp == nil || resp.Msg == nil {
		return nil, false, fmt.Errorf("list organization nodes: empty response")
	}
	for _, candidate := range resp.Msg.GetItems() {
		if candidate != nil && candidate.GetExternalNodeId() == reg.ExternalNodeID {
			return candidate, false, nil
		}
	}
	if resp.Msg.GetNextPageToken() != "" {
		return nil, false, fmt.Errorf("registered node was not in the returned page and node listing is incomplete")
	}
	return nil, true, nil
}

func remainingSSHAccessCounts(accesses []*nodev1.SSHAccess) (int, int) {
	accounts := make(map[string]struct{}, len(accesses))
	grantCount := 0
	for _, access := range accesses {
		if access == nil {
			continue
		}
		grantCount++
		accounts[access.GetLinuxUser()] = struct{}{}
	}
	return grantCount, len(accounts)
}
