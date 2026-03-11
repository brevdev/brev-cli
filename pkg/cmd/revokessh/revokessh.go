// Package revokessh provides the brev revoke-ssh command for revoking SSH access
// to a registered device by calling the backend API.
package revokessh

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

// RevokeSSHStore defines the store methods needed by the revoke-ssh command.
type RevokeSSHStore interface {
	GetAccessToken() (string, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetOrganizationsByName(name string) ([]entity.Organization, error)
	ListOrganizations() ([]entity.Organization, error)
	GetUserByID(userID string) (*entity.User, error)
	GetOrgRoleAttachments(orgID string) ([]entity.OrgRoleAttachment, error)
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
	var orgFlag string
	var nodeFlag string
	var userFlag string
	var linuxUserFlag string
	var approveFlag bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "revoke-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Revoke SSH access to a node for an org member",
		Long:                  "Revoke SSH access to a node for a member of your organization. Interactive: no flags, prompts for org and which access to revoke. Non-interactive: --org, --user, --linux-user required.",
		Example:               "  brev revoke-ssh\n  brev revoke-ssh --org my-org --node my-node --user user@example.com --linux-user ubuntu --approve",
		RunE: func(cmd *cobra.Command, args []string) error {
			interactive := orgFlag == "" && nodeFlag == "" && userFlag == "" && linuxUserFlag == ""
			opts := revokeSSHOpts{
				interactive:   interactive,
				orgName:       orgFlag,
				nodeName:      nodeFlag,
				userIDOrEmail: userFlag,
				linuxUser:     linuxUserFlag,
				skipConfirm:   approveFlag,
			}
			return runRevokeSSH(cmd.Context(), t, store, opts, defaultRevokeSSHDeps())
		},
	}

	cmd.Flags().StringVarP(&orgFlag, "org", "o", "", "organization name (required in non-interactive mode)")
	cmd.Flags().StringVarP(&nodeFlag, "node", "n", "", "node name (required in non-interactive mode)")
	cmd.Flags().StringVarP(&userFlag, "user", "u", "", "Brev user ID or email to revoke (required in non-interactive mode)")
	cmd.Flags().StringVar(&linuxUserFlag, "linux-user", "", "Linux username on the target node (required in non-interactive mode)")
	cmd.Flags().BoolVar(&approveFlag, "approve", false, "skip confirmation prompt (assume yes)")

	return cmd
}

// revokeSSHOpts carries mode and inputs: when interactive, org/user/linuxUser from prompts; otherwise from flags.
type revokeSSHOpts struct {
	interactive   bool
	orgName       string
	nodeName      string
	userIDOrEmail string
	linuxUser     string
	skipConfirm   bool
}

// runRevokeSSH runs the revoke-ssh flow; the only difference by mode is whether we prompt or use opts.
func runRevokeSSH(ctx context.Context, t *terminal.Terminal, s RevokeSSHStore, opts revokeSSHOpts, deps revokeSSHDeps) error {
	if !opts.interactive {
		if opts.orgName == "" || opts.nodeName == "" || opts.userIDOrEmail == "" || opts.linuxUser == "" {
			return fmt.Errorf("in non-interactive mode --org, --node, --user, and --linux-user are required")
		}
	}

	var org *entity.Organization
	var err error
	if opts.interactive {
		list, listErr := s.ListOrganizations()
		if listErr != nil {
			return breverrors.WrapAndTrace(listErr)
		}
		org, err = helpers.SelectOrganizationInteractive(t, list, deps.prompter)
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
	sshAccess := node.GetSshAccess()
	if len(sshAccess) == 0 {
		t.Vprint("No SSH access entries found on this node.")
		return nil
	}

	var targetUserID, targetLinuxUser string
	var selectedAccess *nodev1.SSHAccess

	if opts.interactive {
		labels := make([]string, len(sshAccess))
		for i, sa := range sshAccess {
			userName := sa.GetUserId()
			if u, err := s.GetUserByID(sa.GetUserId()); err == nil && u != nil {
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
		selectedAccess = sshAccess[selectedIdx]
		targetUserID = selectedAccess.GetUserId()
		targetLinuxUser = selectedAccess.GetLinuxUser()
	} else {
		targetUserID, err = resolveUserID(s, org.ID, opts.userIDOrEmail)
		if err != nil {
			return err
		}
		targetLinuxUser = opts.linuxUser
		var found bool
		for _, sa := range sshAccess {
			if sa.GetUserId() == targetUserID && sa.GetLinuxUser() == targetLinuxUser {
				selectedAccess = sa
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no SSH access entry found for user %q and linux_user %q on this node", targetUserID, targetLinuxUser)
		}
	}

	userDisplay := targetUserID
	if u, err := s.GetUserByID(targetUserID); err == nil && u != nil {
		userDisplay = fmt.Sprintf("%s (%s)", u.Name, targetUserID)
	}

	t.Vprint("")
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint(t.White("  Revoking SSH access"))
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint("")
	if opts.interactive && !opts.skipConfirm {
		t.Vprint(t.Green("  Please confirm before continuing:"))
		t.Vprint("")
	}
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Node:")), t.BoldBlue(node.GetName()+" ("+node.GetExternalNodeId()+")"))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "User:")), t.BoldBlue(userDisplay))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Linux user:")), t.BoldBlue(selectedAccess.GetLinuxUser()))
	t.Vprint("")

	if !opts.skipConfirm {
		confirm := deps.prompter.Select("Proceed?", []string{"Yes, proceed", "No, cancel"})
		if confirm != "Yes, proceed" {
			t.Vprint("Revoke canceled.")
			return nil
		}
	}

	_, err = client.RevokeNodeSSHAccess(ctx, connect.NewRequest(&nodev1.RevokeNodeSSHAccessRequest{
		ExternalNodeId: node.GetExternalNodeId(),
		UserId:         targetUserID,
		LinuxUser:      targetLinuxUser,
	}))
	if err != nil {
		return fmt.Errorf("failed to revoke SSH access: %w", err)
	}

	t.Vprint(t.Green("SSH access revoked."))
	return nil
}

// resolveUserID resolves idOrEmail to a Brev user ID using org members when it looks like an email.
func resolveUserID(s RevokeSSHStore, orgID string, idOrEmail string) (string, error) {
	idOrEmail = strings.TrimSpace(strings.ToLower(idOrEmail))
	if idOrEmail == "" {
		return "", fmt.Errorf("user is required")
	}
	if !strings.Contains(idOrEmail, "@") {
		u, err := s.GetUserByID(idOrEmail)
		if err == nil && u != nil {
			return u.ID, nil
		}
		return idOrEmail, nil
	}
	attachments, err := s.GetOrgRoleAttachments(orgID)
	if err != nil {
		return "", fmt.Errorf("failed to list org members: %w", err)
	}
	for _, a := range attachments {
		u, err := s.GetUserByID(a.Subject)
		if err != nil {
			continue
		}
		if u != nil && strings.ToLower(u.Email) == idOrEmail {
			return u.ID, nil
		}
	}
	return "", fmt.Errorf("no org member found with email %q", idOrEmail)
}
