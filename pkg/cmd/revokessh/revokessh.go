// Package revokessh provides the brev revoke-ssh command for revoking SSH access
// to a registered device by removing a Brev-managed key from authorized_keys.
package revokessh

import (
	"context"
	"fmt"
	"os/user"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// RevokeSSHStore defines the store methods needed by the revoke-ssh command.
type RevokeSSHStore interface {
	GetAccessToken() (string, error)
}

// revokeSSHDeps bundles the side-effecting dependencies of runRevokeSSH so they
// can be replaced in tests.
type revokeSSHDeps struct {
	platform          externalnode.PlatformChecker
	prompter          terminal.Selector
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	currentUser       func() (*user.User, error)
	listBrevKeys      func(u *user.User) ([]register.BrevAuthorizedKey, error)
	removeKeyLine     func(u *user.User, line string) error
}

func defaultRevokeSSHDeps() revokeSSHDeps {
	return revokeSSHDeps{
		platform:          register.LinuxPlatform{},
		prompter:          register.TerminalPrompter{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
		currentUser:       user.Current,
		listBrevKeys:      register.ListBrevAuthorizedKeys,
		removeKeyLine:     register.RemoveAuthorizedKeyLine,
	}
}

func NewCmdRevokeSSH(t *terminal.Terminal, store RevokeSSHStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "revoke-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Revoke SSH access to this device for an org member",
		Long:                  "Revoke SSH access to this registered device for another member of your organization.",
		Example:               "  brev revoke-ssh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRevokeSSH(cmd.Context(), t, store, defaultRevokeSSHDeps())
		},
	}

	return cmd
}

func runRevokeSSH(ctx context.Context, t *terminal.Terminal, s RevokeSSHStore, deps revokeSSHDeps) error {
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev revoke-ssh is only supported on Linux")
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	osUser, err := deps.currentUser()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}

	brevKeys, err := deps.listBrevKeys(osUser)
	if err != nil {
		return fmt.Errorf("failed to read authorized keys: %w", err)
	}

	if len(brevKeys) == 0 {
		t.Vprint("No Brev SSH keys found to revoke.")
		return nil
	}

	// Build selector labels from the installed keys.
	labels := make([]string, len(brevKeys))
	for i, bk := range brevKeys {
		keyPreview := truncateKey(bk.KeyContent, 60)
		if bk.UserID != "" {
			labels[i] = fmt.Sprintf("%s (user: %s)", keyPreview, bk.UserID)
		} else {
			labels[i] = keyPreview
		}
	}

	selected := deps.prompter.Select("Select a Brev SSH key to revoke:", labels)

	selectedIdx := -1
	for i, label := range labels {
		if label == selected {
			selectedIdx = i
			break
		}
	}
	if selectedIdx < 0 {
		return fmt.Errorf("selected item %q did not match any key", selected)
	}

	selectedKey := brevKeys[selectedIdx]

	t.Vprint("")
	t.Vprint(t.Green("Revoking SSH access"))
	t.Vprint("")
	t.Vprintf("  Node: %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Key:  %s\n", truncateKey(selectedKey.KeyContent, 80))
	if selectedKey.UserID != "" {
		t.Vprintf("  User: %s\n", selectedKey.UserID)
	}
	t.Vprint("")

	// Remove the key from authorized_keys first.
	if err := deps.removeKeyLine(osUser, selectedKey.Line); err != nil {
		return fmt.Errorf("failed to remove key from authorized_keys: %w", err)
	}
	t.Vprint("  Brev public key removed from authorized_keys.")

	// If we know the user ID, also revoke server-side.
	if selectedKey.UserID != "" {
		client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
		_, err := client.RevokeNodeSSHAccess(ctx, connect.NewRequest(&nodev1.RevokeNodeSSHAccessRequest{
			ExternalNodeId: reg.ExternalNodeID,
			UserId:         selectedKey.UserID,
			LinuxUser:      osUser.Username,
		}))
		if err != nil {
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: server-side revocation failed: %v", err)))
			t.Vprint("  The key was removed locally but the server may still show access.")
		}
	} else {
		t.Vprint("  Key was old-format (no user ID); skipping server-side revocation.")
	}

	t.Vprint("")
	t.Vprint(t.Green("SSH key revoked."))
	return nil
}

// truncateKey shortens a key string for display, showing the first maxLen
// characters followed by "..." if it's longer.
func truncateKey(key string, maxLen int) string {
	if len(key) <= maxLen {
		return key
	}
	return key[:maxLen] + "..."
}
