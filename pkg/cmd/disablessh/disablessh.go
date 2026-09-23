package disablessh

import (
	"context"
	"fmt"
	"os/user"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/sshcert"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type DisableSSHStore interface {
	GetCurrentUser() (*entity.User, error)
	GetAccessToken() (string, error)
}

type disableSSHDeps struct {
	platform          externalnode.PlatformChecker
	registrationStore register.RegistrationStore
}

func defaultDisableSSHDeps() disableSSHDeps {
	return disableSSHDeps{
		platform:          register.LinuxPlatform{},
		registrationStore: register.NewFileRegistrationStore(),
	}
}

func NewCmdDisableSSH(t *terminal.Terminal, store DisableSSHStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "disable-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Remove Brev SSH access data from this device",
		Long:                  "Removes the Brev certificate authority line and any Brev-managed SSH keys from authorized_keys, revoking SSH access for all users. The node remains registered.",
		Example:               "  brev disable-ssh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDisableSSH(cmd.Context(), t, store, defaultDisableSSHDeps())
		},
	}

	return cmd
}

func runDisableSSH(_ context.Context, t *terminal.Terminal, _ DisableSSHStore, deps disableSSHDeps) error {
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev disable-ssh is only supported on Linux")
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return fmt.Errorf("failed to read registration file: %w", err)
	}

	linuxUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green("Removing SSH certificate authority from this device"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Linux user: %s\n", linuxUser.Username)
	t.Vprint("")

	removed, err := sshcert.RemoveCertAuthorityLine(linuxUser.HomeDir, reg.ExternalNodeID, linuxUser.Username)
	if err != nil {
		return fmt.Errorf("disable SSH failed: %w", err)
	}

	if removed {
		t.Vprint(t.Green("  Certificate authority removed from authorized_keys."))
	} else {
		t.Vprint(t.Yellow("  No certificate authority line found in authorized_keys."))
	}

	// Legacy nodes store per-user keys instead of a cert-authority line.
	// Remove them too; both operations are idempotent no-ops when nothing
	// matches, so running both covers every node mode.
	removedKeys, kerr := register.RemoveBrevAuthorizedKeys(linuxUser)
	switch {
	case kerr != nil:
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to remove Brev SSH keys: %v", kerr)))
	case len(removedKeys) > 0:
		t.Vprintf("%s  Brev SSH keys removed from authorized_keys:\n", t.Green("  ✓"))
		for _, key := range removedKeys {
			t.Vprintf("    - %s\n", key)
		}
	}

	t.Vprint(t.Green("SSH disabled. Run 'brev enable-ssh' to re-enable."))
	return nil
}
