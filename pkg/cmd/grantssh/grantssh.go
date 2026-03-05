// Package grantssh provides the brev grant-ssh command for granting SSH access
// to a registered device for another org member.
package grantssh

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
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
	GetBrevHomePath() (string, error)
	GetAccessToken() (string, error)
	GetOrgRoleAttachments(orgID string) ([]entity.OrgRoleAttachment, error)
	GetUserByID(userID string) (*entity.User, error)
}

// grantSSHDeps bundles the side-effecting dependencies of runGrantSSH so they
// can be replaced in tests.
type grantSSHDeps struct {
	platform          externalnode.PlatformChecker
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
		platform:          register.LinuxPlatform{},
		prompter:          register.TerminalPrompter{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
	}
}

func NewCmdGrantSSH(t *terminal.Terminal, store GrantSSHStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "grant-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Grant SSH access to this device for another org member",
		Long:                  "Grant SSH access to this registered device for another member of your organization.",
		Example:               "  brev grant-ssh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGrantSSH(cmd.Context(), t, store, defaultGrantSSHDeps())
		},
	}

	return cmd
}

func runGrantSSH(ctx context.Context, t *terminal.Terminal, s GrantSSHStore, deps grantSSHDeps) error { //nolint:funlen // grant-ssh flow
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev grant-ssh is only supported on Linux")
	}

	removeCredentialsFile(t, s)

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	currentUser, err := s.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if err := checkSSHEnabled(currentUser.PublicKey); err != nil {
		return err
	}

	osUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}
	linuxUser := osUser.Username

	org, err := s.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return fmt.Errorf("no organization found; please create or join an organization first")
	}

	orgMembers, err := getOrgMembers(currentUser, t, s, org.ID)
	// Resolve user details for each member.
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Build selection list.
	usersToSelect := make([]string, len(orgMembers))
	for i, r := range orgMembers {
		usersToSelect[i] = fmt.Sprintf("%s (%s)", r.user.Name, r.user.Email)
	}

	selected := deps.prompter.Select("Select a user to grant SSH access:", usersToSelect)

	// Find the selected user.
	selectedUser, err := getSelectedUser(usersToSelect, selected, orgMembers)
	if err != nil {
		return err
	}

	t.Vprint("")
	t.Vprint(t.Green("Granting SSH access"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Brev user:  %s (%s)\n", selectedUser.Name, selectedUser.ID)
	t.Vprintf("  Linux user: %s\n", linuxUser)
	t.Vprint("")

	port, err := register.PromptSSHPort(t)
	if err != nil {
		return fmt.Errorf("SSH port: %w", err)
	}

	if err := register.GrantSSHAccessToNode(ctx, t, deps.nodeClients, s, reg, selectedUser, osUser, port); err != nil {
		return fmt.Errorf("grant SSH failed: %w", err)
	}

	t.Vprint(t.Green(fmt.Sprintf("SSH access granted for %s. They can now SSH to this device via: brev shell %s", selectedUser.Name, reg.DisplayName)))
	return nil
}

// checkSSHEnabled verifies that SSH has been enabled on this device by checking
// if the current user's public key is present in authorized_keys.
func checkSSHEnabled(currentUserPubKey string) error {
	currentUserPubKey = strings.TrimSpace(currentUserPubKey)
	if currentUserPubKey == "" {
		return fmt.Errorf("current user does not have a Brev public key")
	}

	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}

	authKeysPath := filepath.Join(u.HomeDir, ".ssh", "authorized_keys")
	existing, err := os.ReadFile(authKeysPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to read authorized_keys, %w", err)
	}

	if !strings.Contains(string(existing), currentUserPubKey) {
		return fmt.Errorf("run 'brev enable-ssh' first")
	}

	return nil
}

func getOrgMembers(currentUser *entity.User, t *terminal.Terminal, s GrantSSHStore, orgId string) ([]resolvedMember, error) {
	attachments, err := s.GetOrgRoleAttachments(orgId)
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

// removeCredentialsFile removes ~/.brev/credentials.json if it exists.
// When granting SSH access to another user, we don't want them to find
// the device owner's auth tokens on disk.
func removeCredentialsFile(t *terminal.Terminal, s GrantSSHStore) {
	brevHome, err := s.GetBrevHomePath()
	if err != nil {
		return
	}
	credsPath := filepath.Join(brevHome, "credentials.json")
	if err := os.Remove(credsPath); err != nil {
		if !os.IsNotExist(err) {
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to remove credentials file: %v\n  It is recommended to remove this file yourself so that this user does not see any sensitive tokens:\n    rm %s", err, credsPath)))
		}
		return
	}
	t.Vprintf("  Removed %s\n", credsPath)
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
