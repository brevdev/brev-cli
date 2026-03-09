// Package revokessh provides the brev revoke-ssh command for revoking SSH access
// to a registered device by calling the backend API.
package revokessh

import (
	"context"
	"fmt"

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

// RevokeSSHStore defines the store methods needed by the revoke-ssh command.
type RevokeSSHStore interface {
	GetAccessToken() (string, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetUserByID(userID string) (*entity.User, error)
}

// revokeSSHDeps bundles the side-effecting dependencies of runRevokeSSH so they
// can be replaced in tests.
type revokeSSHDeps struct {
	prompter          terminal.Selector
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
}

func defaultRevokeSSHDeps() revokeSSHDeps {
	return revokeSSHDeps{
		prompter:          register.TerminalPrompter{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
	}
}

func NewCmdRevokeSSH(t *terminal.Terminal, store RevokeSSHStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "revoke-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Revoke SSH access to a node for an org member",
		Long:                  "Revoke SSH access to a node for a member of your organization. Can be run from any machine.",
		Example:               "  brev revoke-ssh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRevokeSSH(cmd.Context(), t, store, defaultRevokeSSHDeps())
		},
	}

	return cmd
}

func runRevokeSSH(ctx context.Context, t *terminal.Terminal, s RevokeSSHStore, deps revokeSSHDeps) error { //nolint:funlen // revoke-ssh flow
	org, err := s.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return fmt.Errorf("no organization found; please create or join an organization first")
	}

	node, err := register.ResolveNode(ctx, deps.prompter, deps.nodeClients, s, deps.registrationStore, org.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Fetch node details to get the SSH access list.
	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	nodeResp, err := client.GetNode(ctx, connect.NewRequest(&nodev1.GetNodeRequest{
		ExternalNodeId: node.ExternalNodeID,
		OrganizationId: org.ID,
	}))
	if err != nil {
		return fmt.Errorf("failed to fetch node details: %w", err)
	}

	sshAccess := nodeResp.Msg.GetExternalNode().GetSshAccess()
	if len(sshAccess) == 0 {
		t.Vprint("No SSH access entries found on this node.")
		return nil
	}

	// Build selection labels, resolving user names where possible.
	labels := make([]string, len(sshAccess))
	for i, sa := range sshAccess {
		userName := sa.GetUserId()
		if u, err := s.GetUserByID(sa.GetUserId()); err == nil {
			userName = fmt.Sprintf("%s (%s)", u.Name, sa.GetUserId())
		}
		labels[i] = fmt.Sprintf("%s, linux_user: %s", userName, sa.GetLinuxUser())
	}

	selected := deps.prompter.Select("Select SSH access to revoke:", labels)

	selectedIdx := -1
	for i, label := range labels {
		if label == selected {
			selectedIdx = i
			break
		}
	}
	if selectedIdx < 0 {
		return fmt.Errorf("selected item %q did not match any access entry", selected)
	}

	selectedAccess := sshAccess[selectedIdx]

	t.Vprint("")
	t.Vprint(t.Green("Revoking SSH access"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", node.DisplayName, node.ExternalNodeID)
	t.Vprintf("  User:       %s\n", selectedAccess.GetUserId())
	t.Vprintf("  Linux user: %s\n", selectedAccess.GetLinuxUser())
	t.Vprint("")

	_, err = client.RevokeNodeSSHAccess(ctx, connect.NewRequest(&nodev1.RevokeNodeSSHAccessRequest{
		ExternalNodeId: node.ExternalNodeID,
		UserId:         selectedAccess.GetUserId(),
		LinuxUser:      selectedAccess.GetLinuxUser(),
	}))
	if err != nil {
		return fmt.Errorf("failed to revoke SSH access: %w", err)
	}

	t.Vprint(t.Green("SSH access revoked."))
	return nil
}
