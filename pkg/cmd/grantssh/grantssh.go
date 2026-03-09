// Package grantssh provides the brev grant-ssh command for granting SSH access
// to a registered device for another org member.
package grantssh

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

// GrantSSHStore defines the store methods needed by the grant-ssh command.
type GrantSSHStore interface {
	GetCurrentUser() (*entity.User, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetAccessToken() (string, error)
	GetOrgRoleAttachments(orgID string) ([]entity.OrgRoleAttachment, error)
	GetUserByID(userID string) (*entity.User, error)
}

// grantSSHDeps bundles the side-effecting dependencies of runGrantSSH so they
// can be replaced in tests.
type grantSSHDeps struct {
	prompter          terminal.Selector
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
}

type resolvedMember struct {
	user       *entity.User
	attachment entity.OrgRoleAttachment
}

func defaultGrantSSHDeps() grantSSHDeps {
	return grantSSHDeps{
		prompter:          register.TerminalPrompter{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
	}
}

func NewCmdGrantSSH(t *terminal.Terminal, store GrantSSHStore) *cobra.Command {
	var linuxUser string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "grant-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Grant SSH access to a node for another org member",
		Long:                  "Grant SSH access to a node for another member of your organization",
		Example:               "  brev grant-ssh --linux-user ubuntu",
		RunE: func(cmd *cobra.Command, args []string) error {
			deps := defaultGrantSSHDeps()
			if linuxUser == "" {
				linuxUser = register.GetCachedLinuxUser()
			}
			if linuxUser == "" {
				return fmt.Errorf("--linux-user is required (no cached value found)")
			}
			register.SaveCachedLinuxUser(linuxUser)
			return runGrantSSH(cmd.Context(), t, store, deps, linuxUser)
		},
	}

	cmd.Flags().StringVar(&linuxUser, "linux-user", "", "Linux username on the target node")

	return cmd
}

func runGrantSSH(ctx context.Context, t *terminal.Terminal, s GrantSSHStore, deps grantSSHDeps, linuxUser string) error { //nolint:funlen // grant-ssh flow
	currentUser, err := s.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

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

	orgMembers, err := getOrgMembers(currentUser, t, s, org.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Build selection list.
	usersToSelect := make([]string, len(orgMembers))
	for i, r := range orgMembers {
		usersToSelect[i] = fmt.Sprintf("%s (%s)", r.user.Name, r.user.Email)
	}

	selected := deps.prompter.Select("Select a user to grant SSH access:", usersToSelect)

	selectedUser, err := getSelectedUser(usersToSelect, selected, orgMembers)
	if err != nil {
		return err
	}

	t.Vprint("")
	t.Vprint(t.Green("Granting SSH access"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", node.DisplayName, node.ExternalNodeID)
	t.Vprintf("  Brev user:  %s (%s)\n", selectedUser.Name, selectedUser.ID)
	t.Vprintf("  Linux user: %s\n", linuxUser)
	t.Vprint("")

	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	_, err = client.GrantNodeSSHAccess(ctx, connect.NewRequest(&nodev1.GrantNodeSSHAccessRequest{
		ExternalNodeId: node.ExternalNodeID,
		UserId:         selectedUser.ID,
		LinuxUser:      linuxUser,
	}))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green(fmt.Sprintf("SSH access granted for %s. They can now SSH to this device via: brev shell %s", selectedUser.Name, node.DisplayName)))
	return nil
}

func getOrgMembers(currentUser *entity.User, t *terminal.Terminal, s GrantSSHStore, orgID string) ([]resolvedMember, error) {
	attachments, err := s.GetOrgRoleAttachments(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch org members: %w", err)
	}

	// Filter out current user.
	var otherMembers []entity.OrgRoleAttachment
	for _, a := range attachments {
		if a.Subject != currentUser.ID {
			otherMembers = append(otherMembers, a)
		}
	}

	if len(otherMembers) == 0 {
		return nil, fmt.Errorf("no other members found in current organization")
	}
	var resolved []resolvedMember
	for _, m := range otherMembers {
		memberUser, err := s.GetUserByID(m.Subject)
		if err != nil {
			t.Vprintf("  Warning: could not resolve user %s: %v\n", m.Subject, err)
			continue
		}
		resolved = append(resolved, resolvedMember{user: memberUser, attachment: m})
	}

	if len(resolved) == 0 {
		return nil, fmt.Errorf("could not resolve any org member details")
	}

	return resolved, nil
}

func getSelectedUser(usersToSelect []string, selected string, orgMembers []resolvedMember) (*entity.User, error) {
	selectedIdx := -1
	for i, userSelection := range usersToSelect {
		if userSelection == selected {
			selectedIdx = i
			break
		}
	}
	if selectedIdx < 0 {
		return nil, fmt.Errorf("selected item %q did not match any org member", selected)
	}
	selectedUser := orgMembers[selectedIdx].user
	return selectedUser, nil
}
