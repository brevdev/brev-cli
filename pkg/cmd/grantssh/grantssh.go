// Package grantssh provides the brev grant-ssh command for granting SSH access
// to a registered device for another org member.
package grantssh

import (
	"context"
	"fmt"
	"strings"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/externalnode/helpers"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// GrantSSHStore defines the store methods needed by the grant-ssh command.
type GrantSSHStore interface {
	GetCurrentUser() (*entity.User, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetOrganizationsByName(name string) ([]entity.Organization, error)
	ListOrganizations() ([]entity.Organization, error)
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
	var orgFlag string
	var nodeFlag string
	var userFlag string
	var linuxUser string
	var approveFlag bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "grant-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Grant SSH access to a node for another org member",
		Long:                  "Grant SSH access to a node for another member of your organization. Interactive: no flags, prompts for org and user. Non-interactive: --org, --user, --linux-user required.",
		Example:               "  brev grant-ssh\n  brev grant-ssh --org my-org --node my-node --user user@example.com --linux-user ubuntu --approve",
		RunE: func(cmd *cobra.Command, args []string) error {
			interactive := orgFlag == "" && nodeFlag == "" && userFlag == ""
			opts := grantSSHOpts{
				interactive:   interactive,
				orgName:       orgFlag,
				nodeName:      nodeFlag,
				userIDOrEmail: userFlag,
				linuxUser:     linuxUser,
				skipConfirm:   approveFlag,
			}
			return runGrantSSH(cmd.Context(), t, store, opts, defaultGrantSSHDeps())
		},
	}

	cmd.Flags().StringVarP(&orgFlag, "org", "o", "", "organization name (required in non-interactive mode)")
	cmd.Flags().StringVarP(&nodeFlag, "node", "n", "", "node name (required in non-interactive mode)")
	cmd.Flags().StringVarP(&userFlag, "user", "u", "", "Brev user ID or email to grant (required in non-interactive mode)")
	cmd.Flags().StringVar(&linuxUser, "linux-user", "", "Linux username on the target node (required in non-interactive mode)")
	cmd.Flags().BoolVar(&approveFlag, "approve", false, "skip confirmation prompt (assume yes)")

	return cmd
}

// grantSSHOpts carries mode and inputs: when interactive, org/user/linuxUser from prompts; otherwise from flags.
type grantSSHOpts struct {
	interactive   bool
	orgName       string
	nodeName      string
	userIDOrEmail string
	linuxUser     string
	skipConfirm   bool
}

// runGrantSSH runs the grant-ssh flow; the only difference by mode is whether we prompt or use opts.
func runGrantSSH(ctx context.Context, t *terminal.Terminal, s GrantSSHStore, opts grantSSHOpts, deps grantSSHDeps) error {
	// Run through the login flow
	currentUser, err := s.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if !opts.interactive {
		if opts.orgName == "" || opts.nodeName == "" || opts.userIDOrEmail == "" {
			return fmt.Errorf("in non-interactive mode --org, --node, and --user are required")
		}
		if opts.linuxUser == "" {
			return fmt.Errorf("--linux-user is required in non-interactive mode (no cached value found)")
		}
	}

	// Capture the target organization
	var org *entity.Organization
	if opts.interactive {
		allOrgs, listErr := s.ListOrganizations()
		if listErr != nil {
			return breverrors.WrapAndTrace(listErr)
		}
		org, err = helpers.SelectOrganizationInteractive(t, allOrgs, deps.prompter)
	} else {
		org, err = helpers.ResolveOrgByName(s, opts.orgName)
	}
	if err != nil {
		return err
	}

	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())

	// Capture the target node
	var node *nodev1.ExternalNode
	if opts.interactive {
		resp, listErr := client.ListNodes(ctx, connect.NewRequest(&nodev1.ListNodesRequest{
			OrganizationId: org.ID,
		}))
		if listErr != nil {
			return breverrors.WrapAndTrace(listErr)
		}
		nodes := resp.Msg.GetItems()
		if len(nodes) == 0 {
			return fmt.Errorf("no nodes found in organization")
		}
		node, err = register.SelectNodeFromList(ctx, t, deps.prompter, deps.registrationStore, nodes)
	} else {
		node, err = register.ResolveNodeByName(ctx, deps.nodeClients, s, org.ID, opts.nodeName)
	}
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	orgMembers, err := getOrgMembers(currentUser, t, s, org.ID)
	if err != nil {
		return err
	}

	var selectedUser *entity.User
	var linuxUser string
	if opts.interactive {
		usersToSelect := make([]string, len(orgMembers))
		for i, r := range orgMembers {
			usersToSelect[i] = fmt.Sprintf("%s (%s)", r.user.Name, r.user.Email)
		}
		selected := deps.prompter.Select("Select a user to grant SSH access:", usersToSelect)
		selectedUser, err = getSelectedUser(usersToSelect, selected, orgMembers)
		if err != nil {
			return err
		}
		linuxUserOptions := linuxUsersFromNode(node)
		if len(linuxUserOptions) > 0 {
			t.Vprint("")
			linuxUser = deps.prompter.Select("Select Linux user on the node", linuxUserOptions)
		} else {
			return fmt.Errorf("no Linux users on this node yet; run with --linux-user to specify one (e.g. after enable-ssh on the node)")
		}
	} else {
		selectedUser, err = findUserByIDOrEmail(orgMembers, opts.userIDOrEmail)
		if err != nil {
			return err
		}
		linuxUser = opts.linuxUser
	}

	t.Vprint("")
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint(t.White("  Granting SSH access"))
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint("")
	if opts.interactive && !opts.skipConfirm {
		t.Vprint(t.Green("  Please confirm before continuing:"))
		t.Vprint("")
	}
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Node:")), t.BoldBlue(node.GetName()+" ("+node.GetExternalNodeId()+")"))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Brev user:")), t.BoldBlue(selectedUser.Name+" ("+selectedUser.ID+")"))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Linux user:")), t.BoldBlue(linuxUser))
	t.Vprint("")

	if !opts.skipConfirm {
		confirm := deps.prompter.Select("Proceed?", []string{"Yes, proceed", "No, cancel"})
		if confirm != "Yes, proceed" {
			t.Vprint("Grant canceled.")
			return nil
		}
	}

	_, err = client.GrantNodeSSHAccess(ctx, connect.NewRequest(&nodev1.GrantNodeSSHAccessRequest{
		ExternalNodeId: node.GetExternalNodeId(),
		UserId:         selectedUser.ID,
		LinuxUser:      linuxUser,
	}))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green(fmt.Sprintf("SSH access granted for %s. They can now SSH to this device via: brev shell %s", selectedUser.Name, node.GetName())))
	return nil
}

// linuxUsersFromNode returns distinct Linux users from the node's existing SSH access (for picker).
func linuxUsersFromNode(node *nodev1.ExternalNode) []string {
	if node == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, sa := range node.GetSshAccess() {
		u := sa.GetLinuxUser()
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

func findUserByIDOrEmail(members []resolvedMember, idOrEmail string) (*entity.User, error) {
	idOrEmail = strings.TrimSpace(strings.ToLower(idOrEmail))
	for _, r := range members {
		if strings.ToLower(r.user.ID) == idOrEmail || strings.ToLower(r.user.Email) == idOrEmail {
			return r.user, nil
		}
	}
	return nil, fmt.Errorf("no org member found matching %q", idOrEmail)
}

func getOrgMembers(currentUser *entity.User, t *terminal.Terminal, s GrantSSHStore, orgID string) ([]resolvedMember, error) {
	attachments, err := s.GetOrgRoleAttachments(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch org members: %w", err)
	}

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
	return orgMembers[selectedIdx].user, nil
}
