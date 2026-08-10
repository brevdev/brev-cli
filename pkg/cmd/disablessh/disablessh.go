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
	"github.com/brevdev/brev-cli/pkg/sudo"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// DisableSSHStore defines the authenticated store methods needed by disable-ssh.
type DisableSSHStore interface {
	GetCurrentUser() (*entity.User, error)
	GetAccessToken() (string, error)
}

type disableSSHDeps struct {
	platform          externalnode.PlatformChecker
	confirmer         terminal.Confirmer
	gater             sudo.Gater
	tunnel            register.NetBirdConnector
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	keyCleaner        localKeyCleaner
}

func defaultDisableSSHDeps() disableSSHDeps {
	return disableSSHDeps{
		platform:          register.LinuxPlatform{},
		confirmer:         register.TerminalPrompter{},
		gater:             sudo.Default,
		tunnel:            register.Netbird{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
		keyCleaner:        newPrivilegedLocalKeyCleaner(),
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
		Short:                 "Disable all Brev-managed SSH access on this node",
		Long:                  "Disable every Brev-managed SSH credential on this joined node without changing Brev network membership or the SSH daemon.",
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
) error { //nolint:funlen // Ordered teardown state machine is intentionally explicit.
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev disable-ssh is only supported on Linux")
	}

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
	if _, err := store.GetCurrentUser(); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	node, err := register.FetchRegisteredNode(ctx, deps.nodeClients, store, reg)
	if err != nil {
		return fmt.Errorf("disable SSH failed: %w", err)
	}
	accesses := snapshotSSHAccess(node.GetSshAccess())
	linuxAccounts := distinctLinuxAccountCount(accesses)

	t.Vprint("")
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint(t.White("  Disabling Brev-managed SSH access"))
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint("")
	t.Vprintf("  Node:           %s (%s)\n", node.GetName(), node.GetExternalNodeId())
	t.Vprintf("  SSH grants:    %d\n", len(accesses))
	t.Vprintf("  Linux accounts: %d\n", linuxAccounts)
	t.Vprint("")
	if warnings == nil {
		warnings = io.Discard
	}
	_, _ = fmt.Fprintln(warnings, "Warning: this is a node-wide operation that removes all Brev-managed SSH credentials on this node.")
	_, _ = fmt.Fprintln(warnings, "Warning: active SSH sessions are not forcibly terminated.")

	if !skipConfirm && !deps.confirmer.ConfirmYesNo("Disable all Brev-managed SSH access on this node?") {
		t.Vprint("Disable SSH canceled.")
		return nil
	}

	if err := deps.gater.Gate(t, deps.confirmer, "Node-wide Brev SSH cleanup", true); err != nil {
		return fmt.Errorf("sudo issue: %w", err)
	}

	if len(accesses) > 0 {
		if err := deps.tunnel.EnsureConnected(ctx); err != nil {
			return fmt.Errorf("disable SSH requires a connected Brev tunnel: %w", err)
		}
		client := deps.nodeClients.NewNodeClient(store, config.GlobalConfig.GetBrevPublicAPIURL())
		if err := revokeSSHAccesses(ctx, client, reg.ExternalNodeID, accesses); err != nil {
			return err
		}
	}

	result, err := deps.keyCleaner.RemoveBrevKeys(ctx)
	if err != nil {
		return fmt.Errorf("disable SSH local key cleanup incomplete: %w", err)
	}
	t.Vprintf("%s  SSH access disabled: %d keys removed; %d accounts changed.\n", t.Green("  ✓"), result.KeysRemoved, result.AccountsChanged)
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
		return fmt.Errorf("disable SSH backend cleanup incomplete: %w", err)
	}
	return nil
}

func snapshotSSHAccess(accesses []*nodev1.SSHAccess) []*nodev1.SSHAccess {
	snapshot := make([]*nodev1.SSHAccess, 0, len(accesses))
	for _, access := range accesses {
		if access != nil {
			snapshot = append(snapshot, access)
		}
	}
	return snapshot
}

func distinctLinuxAccountCount(accesses []*nodev1.SSHAccess) int {
	accounts := make(map[string]struct{}, len(accesses))
	for _, access := range accesses {
		accounts[access.GetLinuxUser()] = struct{}{}
	}
	return len(accounts)
}
