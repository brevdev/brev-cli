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

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/enablessh"
	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
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

// PlatformChecker checks whether the current platform is supported.
type PlatformChecker interface {
	IsCompatible() bool
}

// Selector prompts the user to choose from a list of items.
type Selector interface {
	Select(label string, items []string) string
}

// NodeClientFactory creates ConnectRPC ExternalNodeService clients.
type NodeClientFactory interface {
	NewNodeClient(provider register.TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient
}

// grantSSHDeps bundles the side-effecting dependencies of runGrantSSH so they
// can be replaced in tests.
type grantSSHDeps struct {
	platform          PlatformChecker
	prompter          Selector
	nodeClients       NodeClientFactory
	registrationStore register.RegistrationStore
}

type resolvedMember struct {
	user       *entity.User
	attachment entity.OrgRoleAttachment
}

func defaultGrantSSHDeps(brevHome string) grantSSHDeps {
	return grantSSHDeps{
		platform:          register.LinuxPlatform{},
		prompter:          register.TerminalPrompter{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(brevHome),
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
			brevHome, err := store.GetBrevHomePath()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return runGrantSSH(cmd.Context(), t, store, defaultGrantSSHDeps(brevHome))
		},
	}

	return cmd
}

func runGrantSSH(ctx context.Context, t *terminal.Terminal, s GrantSSHStore, deps grantSSHDeps) error { //nolint:funlen // grant-ssh flow
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev grant-ssh is only supported on Linux")
	}

	reg, err := getRegistration(deps)
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

	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}
	linuxUser := u.Username

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
	items := make([]string, len(orgMembers))
	for i, r := range orgMembers {
		items[i] = fmt.Sprintf("%s (%s)", r.user.Name, r.user.Email)
	}

	selected := deps.prompter.Select("Select a user to grant SSH access:", items)

	// Find the selected user.
	var selectedIdx int
	for i, item := range items {
		if item == selected {
			selectedIdx = i
			break
		}
	}
	selectedUser := orgMembers[selectedIdx].user

	t.Vprint("")
	t.Vprint(t.Green("Granting SSH access"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Brev user:  %s (%s)\n", selectedUser.Name, selectedUser.ID)
	t.Vprintf("  Linux user: %s\n", linuxUser)
	t.Vprint("")

	if selectedUser.PublicKey != "" {
		if err := enablessh.InstallAuthorizedKey(u, selectedUser.PublicKey); err != nil {
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to install SSH public key: %v", err)))
		} else {
			t.Vprint("  Brev public key added to authorized_keys.")
		}
	}

	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	if _, err := client.GrantNodeSSHAccess(ctx, connect.NewRequest(&nodev1.GrantNodeSSHAccessRequest{
		ExternalNodeId: reg.ExternalNodeID,
		UserId:         selectedUser.ID,
		LinuxUser:      linuxUser,
	})); err != nil {
		return fmt.Errorf("failed to grant SSH access: %w", err)
	}

	t.Vprint(t.Green(fmt.Sprintf("SSH access granted for %s. They can now SSH to this device via: brev shell %s", selectedUser.Name, reg.DisplayName)))
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

func getRegistration(deps grantSSHDeps) (*register.DeviceRegistration, error) {
	registered, err := deps.registrationStore.Exists()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !registered {
		return nil, fmt.Errorf("no registration found; this machine does not appear to be registered\nRun 'brev register' to register your device first")
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to read registration file: %w", err)
	}
	return reg, nil
}

// checkSSHEnabled verifies that SSH has been enabled on this device by checking
// if the current user's public key is present in authorized_keys.
func checkSSHEnabled(currentUserPubKey string) error {
	currentUserPubKey = strings.TrimSpace(currentUserPubKey)
	if currentUserPubKey == "" {
		return fmt.Errorf("SSH has not been enabled on this device. Run 'brev enable-ssh' first.")
	}

	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}

	authKeysPath := filepath.Join(u.HomeDir, ".ssh", "authorized_keys")
	existing, err := os.ReadFile(authKeysPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("SSH has not been enabled on this device. Run 'brev enable-ssh' first.")
	}

	if !strings.Contains(string(existing), currentUserPubKey) {
		return fmt.Errorf("SSH has not been enabled on this device. Run 'brev enable-ssh' first.")
	}

	return nil
}
