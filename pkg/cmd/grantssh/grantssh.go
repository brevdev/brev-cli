// Package grantssh provides the brev grant-ssh command for granting SSH access
// to a registered device for another org member.
package grantssh

import (
	"context"
	"fmt"
	"os/user"
	"strings"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/auth"
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
	auth.APIKeyAuthStore
	GetCurrentUser() (*entity.User, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetOrganizationsByName(name string) ([]entity.Organization, error)
	ListOrganizations() ([]entity.Organization, error)
	GetAccessToken() (string, error)
	ListOrganizationMembers(ctx context.Context, orgID string) ([]*nodev1.OrganizationMember, error)
	GetUserByID(userID string) (*entity.User, error)
}

// grantSSHDeps bundles the side-effecting dependencies of runGrantSSH so they
// can be replaced in tests.
type grantSSHDeps struct {
	prompter          terminal.Selector
	promptLinuxUser   func(*terminal.Terminal, string) (string, error)
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	currentUser       func() (*user.User, error)
}

type resolvedMember struct {
	user *entity.User
}

func defaultGrantSSHDeps() grantSSHDeps {
	return grantSSHDeps{
		prompter:          register.TerminalPrompter{},
		promptLinuxUser:   register.PromptLinuxUsername,
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
		currentUser:       user.Current,
	}
}

func NewCmdGrantSSH(t *terminal.Terminal, store GrantSSHStore) *cobra.Command {
	var orgFlag string
	var nodeFlag string
	var userFlag string
	var linuxUser string
	var portIDFlag string
	var approveFlag bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "grant-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Grant SSH access to a node for another org member",
		Long:                  "Grant SSH access to a node for another member of your organization. Interactive: no flags, prompts for org (unless API-key auth is active), node, port, user, and Linux user. Non-interactive: --node, --user, and --port-id are required; --org is also required unless API-key auth is active. The Linux user defaults to the current user and can be changed with --linux-user.",
		Example:               "  brev grant-ssh\n  brev grant-ssh --org my-org --node my-node --user user@example.com --port-id port_abc --approve\n  brev grant-ssh --node my-node --user user@example.com --linux-user ubuntu --port-id port_abc --approve --api-key <api-key>",
		RunE: func(cmd *cobra.Command, args []string) error {
			interactive := orgFlag == "" && nodeFlag == "" && userFlag == "" && linuxUser == "" && portIDFlag == ""
			opts := grantSSHOpts{
				interactive:   interactive,
				orgName:       orgFlag,
				nodeName:      nodeFlag,
				userIDOrEmail: userFlag,
				linuxUser:     linuxUser,
				portID:        portIDFlag,
				skipConfirm:   approveFlag,
			}
			return runGrantSSH(cmd.Context(), t, store, opts, defaultGrantSSHDeps())
		},
	}

	cmd.Flags().StringVarP(&orgFlag, "org", "o", "", "organization name (required in non-interactive mode unless using API-key auth)")
	cmd.Flags().StringVarP(&nodeFlag, "node", "n", "", "node name (required in non-interactive mode)")
	cmd.Flags().StringVarP(&userFlag, "user", "u", "", "Brev user ID or email to grant (required in non-interactive mode)")
	cmd.Flags().StringVar(&linuxUser, "linux-user", "", "Linux username on the target node (defaults to the current user)")
	cmd.Flags().StringVar(&portIDFlag, "port-id", "", "Brev port ID to grant access on (required in non-interactive mode)")
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
	portID        string
	skipConfirm   bool
}

// runGrantSSH runs the grant-ssh flow; the only difference by mode is whether we prompt or use opts.
func runGrantSSH(ctx context.Context, t *terminal.Terminal, s GrantSSHStore, opts grantSSHOpts, deps grantSSHDeps) error { //nolint:gocognit,gocyclo,funlen // ok
	apiKeyAuth := auth.IsAPIKeyAuthStore(s)
	if !opts.interactive {
		if opts.nodeName == "" || opts.userIDOrEmail == "" {
			return fmt.Errorf("in non-interactive mode --node and --user are required")
		}
		if opts.orgName == "" && !apiKeyAuth {
			return fmt.Errorf("in non-interactive mode --org is required unless using API-key auth")
		}
		if opts.portID == "" {
			return fmt.Errorf("--port-id is required in non-interactive mode")
		}
	}

	var org *entity.Organization
	var err error
	switch {
	case apiKeyAuth:
		org, err = register.ResolveOrgForAPIKey(s, opts.orgName)
	case opts.interactive:
		allOrgs, listErr := s.ListOrganizations()
		if listErr != nil {
			return breverrors.WrapAndTrace(listErr)
		}
		org, err = helpers.SelectOrganizationInteractive(t, allOrgs, deps.prompter)
	default:
		org, err = helpers.ResolveOrgByName(s, opts.orgName)
	}
	if err != nil {
		return err //nolint:wrapcheck // do not present stack trace for this error
	}

	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())

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
		node, err = helpers.ResolveNodeByName(ctx, client, org.ID, opts.nodeName)
	}
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	orgMembers, err := getOrgMembers(ctx, t, s, org.ID)
	if err != nil {
		return err
	}

	brevPortID, portLabel, err := resolveGrantPort(ctx, t, opts, deps, node)
	if err != nil {
		return err
	}

	var selectedUser *entity.User
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
	} else {
		selectedUser, err = findUserByIDOrEmail(orgMembers, opts.userIDOrEmail)
		if err != nil {
			return err
		}
	}

	linuxUser, err := resolveLinuxUsername(opts.linuxUser, deps.currentUser)
	if err != nil {
		return err
	}
	if opts.interactive {
		t.Vprint("")
		linuxUser, err = deps.promptLinuxUser(t, linuxUser)
		if err != nil {
			return fmt.Errorf("reading Linux username: %w", err)
		}
		linuxUser = strings.TrimSpace(linuxUser)
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
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Port:")), t.BoldBlue(portLabel))
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
		PortId:         brevPortID,
		UserId:         selectedUser.ID,
		LinuxUser:      linuxUser,
	}))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint("")
	t.Vprint(t.Green(fmt.Sprintf("SSH access granted for %s. They can now SSH to this device via: brev shell %s", selectedUser.Name, node.GetName())))
	t.Vprint("")
	return nil
}

// resolveGrantPort returns the Brev port ID and display label for the grant target.
func resolveGrantPort(ctx context.Context, t *terminal.Terminal, opts grantSSHOpts, deps grantSSHDeps, node *nodev1.ExternalNode) (portID, portLabel string, err error) {
	if !opts.interactive {
		p := findPortByID(node, opts.portID)
		if p == nil {
			return "", "", fmt.Errorf("no port with id %q on node %q", opts.portID, node.GetName())
		}
		return p.GetPortId(), register.FormatPortLabel(p), nil
	}

	ports := node.GetPorts()
	if len(ports) == 0 {
		return "", "", fmt.Errorf("no ports found on node %q", node.GetName())
	}
	selected, selErr := register.SelectPortFromList(ctx, t, deps.prompter, ports)
	if selErr != nil {
		return "", "", breverrors.WrapAndTrace(selErr)
	}
	return selected.GetPortId(), register.FormatPortLabel(selected), nil
}

func findPortByID(node *nodev1.ExternalNode, portID string) *nodev1.Port {
	for _, p := range node.GetPorts() {
		if p.GetPortId() == portID {
			return p
		}
	}
	return nil
}

func resolveLinuxUsername(linuxUsername string, currentUser func() (*user.User, error)) (string, error) {
	linuxUsername = strings.TrimSpace(linuxUsername)
	if linuxUsername != "" {
		return linuxUsername, nil
	}
	linuxUser, err := currentUser()
	if err != nil {
		return "", fmt.Errorf("failed to determine current Linux user: %w", err)
	}
	linuxUsername = strings.TrimSpace(linuxUser.Username)
	if linuxUsername == "" {
		return "", fmt.Errorf("failed to determine current Linux user: username is empty")
	}
	return linuxUsername, nil
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

func getOrgMembers(ctx context.Context, t *terminal.Terminal, s GrantSSHStore, orgID string) ([]resolvedMember, error) {
	members, err := s.ListOrganizationMembers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch org members: %w", err)
	}

	// The current user is selectable: granting yourself SSH access is the
	// normal flow for a fresh node (no SSH access entries yet).
	var resolved []resolvedMember
	for _, m := range members {
		memberUser, err := s.GetUserByID(m.GetUserId())
		if err != nil {
			t.Vprintf("  Warning: could not resolve user %s: %v\n", m.GetUserId(), err)
			continue
		}
		resolved = append(resolved, resolvedMember{user: memberUser})
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
