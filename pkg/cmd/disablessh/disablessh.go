// Package disablessh provides the node-wide brev disable-ssh command.
package disablessh

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
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// DisableSSHStore defines the authenticated store methods needed by disable-ssh.
type DisableSSHStore interface {
	GetCurrentUser() (*entity.User, error)
	GetAccessToken() (string, error)
}

type disableSSHDeps struct {
	confirmer         terminal.Confirmer
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
}

func defaultDisableSSHDeps() disableSSHDeps {
	return disableSSHDeps{
		confirmer:         register.TerminalPrompter{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
	}
}

// NewCmdDisableSSH creates the canonical node-wide disable-ssh command.
func NewCmdDisableSSH(t *terminal.Terminal, store DisableSSHStore) *cobra.Command {
	return newCmdDisableSSH(t, store, defaultDisableSSHDeps())
}

func newCmdDisableSSH(t *terminal.Terminal, store DisableSSHStore, deps disableSSHDeps) *cobra.Command {
	var approveFlag bool
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "disable-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Revoke all Brev SSH access grants on this node",
		Long:                  "Revoke every Brev SSH access grant on this joined node without changing Brev network membership or the SSH daemon.",
		Example:               "  brev disable-ssh\n  brev disable-ssh --approve",
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDisableSSH(cmd.Context(), t, cmd.ErrOrStderr(), store, deps, approveFlag)
		},
	}
	cmd.Flags().BoolVar(&approveFlag, "approve", false, "skip confirmation prompt (assume yes)")
	return cmd
}

func runDisableSSH(
	ctx context.Context,
	t *terminal.Terminal,
	warnings io.Writer,
	store DisableSSHStore,
	deps disableSSHDeps,
	skipConfirm bool,
) error { //nolint:funlen // Keep the node-wide confirmation and revocation flow linear.
	exists, err := deps.registrationStore.Exists()
	if err != nil {
		return fmt.Errorf("check joined-device registration: %w", err)
	}
	if !exists {
		return breverrors.New(`This machine has not joined a Brev network; run "brev join" first.`)
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return fmt.Errorf("read joined-device registration: %w", err)
	}
	currentUser, err := store.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if currentUser == nil || currentUser.ID == "" {
		return fmt.Errorf("get current Brev user: missing user ID")
	}

	node, err := register.FetchRegisteredNode(ctx, deps.nodeClients, store, reg)
	if err != nil {
		return fmt.Errorf("disable SSH failed: %w", err)
	}
	accesses := snapshotSSHAccessForRevocation(node.GetSshAccess(), currentUser.ID)

	t.Vprint("")
	t.Vprint(t.White("════════════════════════════════════════════"))
	t.Vprint(t.White("  Disabling Brev SSH access"))
	t.Vprint(t.White("════════════════════════════════════════════"))
	t.Vprint("")
	t.Vprintf("  Node:        %s (%s)\n", node.GetName(), node.GetExternalNodeId())
	t.Vprintf("  SSH grants:  %d\n", len(accesses))
	t.Vprint("")
	if len(accesses) == 0 {
		t.Vprint(t.Green("No SSH access grants to revoke."))
		return nil
	}

	if warnings == nil {
		warnings = io.Discard
	}
	_, _ = fmt.Fprintln(warnings, "Warning: this is a node-wide operation that revokes all Brev SSH access grants on this node.")
	_, _ = fmt.Fprintln(warnings, "Warning: active SSH sessions are not forcibly terminated.")

	if !skipConfirm && !deps.confirmer.ConfirmYesNo("Disable all Brev-managed SSH access on this node?") {
		t.Vprint("Disable SSH canceled.")
		return nil
	}

	client := deps.nodeClients.NewNodeClient(store, config.GlobalConfig.GetBrevPublicAPIURL())
	if err := revokeSSHAccesses(ctx, client, reg.ExternalNodeID, accesses); err != nil {
		return err
	}

	t.Vprintf("%s  SSH access disabled. Grants revoked: %d.\n", t.Green("  ✓"), len(accesses))
	return nil
}

func revokeSSHAccesses(
	ctx context.Context,
	client nodev1connect.ExternalNodeServiceClient,
	nodeID string,
	accesses []*nodev1.SSHAccess,
) error {
	var revokeErrs []error
	for _, access := range accesses {
		_, err := client.RevokeNodeSSHAccess(ctx, connect.NewRequest(&nodev1.RevokeNodeSSHAccessRequest{
			ExternalNodeId: nodeID,
			PortId:         access.GetPortId(),
			UserId:         access.GetUserId(),
			LinuxUser:      access.GetLinuxUser(),
		}))
		if err != nil {
			revokeErrs = append(revokeErrs, fmt.Errorf(
				"revoke SSH access for user %q, Linux account %q, port %q: %w",
				access.GetUserId(),
				access.GetLinuxUser(),
				access.GetPortId(),
				err,
			))
		}
	}
	if err := breverrors.Join(revokeErrs...); err != nil {
		return fmt.Errorf("failed to revoke one or more SSH access grants: %w", err)
	}
	return nil
}

func snapshotSSHAccessForRevocation(accesses []*nodev1.SSHAccess, currentUserID string) []*nodev1.SSHAccess {
	snapshot := make([]*nodev1.SSHAccess, 0, len(accesses))
	currentUserAccesses := make([]*nodev1.SSHAccess, 0, len(accesses))
	for _, access := range accesses {
		if access == nil {
			continue
		}
		if access.GetUserId() == currentUserID {
			currentUserAccesses = append(currentUserAccesses, access)
			continue
		}
		snapshot = append(snapshot, access)
	}
	return append(snapshot, currentUserAccesses...)
}
