// Package enablessh provides the brev enableSSH command for enabling SSH access
// to a registered external node.
package enablessh

import (
	"context"
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// EnableSSHStore defines the store methods needed by the enableSSH command.
type EnableSSHStore interface {
	GetCurrentUser() (*entity.User, error)
	GetBrevHomePath() (string, error)
	GetAccessToken() (string, error)
}

// enableSSHDeps bundles the side-effecting dependencies of runEnableSSH so they
// can be replaced in tests.
type enableSSHDeps struct {
	platform          externalnode.PlatformChecker
	sshd              register.ManagedSSHDaemon
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
}

func defaultEnableSSHDeps(brevHome string) enableSSHDeps {
	return enableSSHDeps{
		platform:          register.LinuxPlatform{},
		sshd:              register.BrevSSHD{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(brevHome),
	}
}

func NewCmdEnableSSH(t *terminal.Terminal, store EnableSSHStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "enable-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Enable SSH access to this registered device",
		Long:                  "Enable SSH access to this registered device for the current Brev user.",
		Example:               "  brev enable-ssh",
		RunE: func(cmd *cobra.Command, args []string) error {
			brevHome, err := store.GetBrevHomePath()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return runEnableSSH(cmd.Context(), t, store, defaultEnableSSHDeps(brevHome))
		},
	}

	return cmd
}

func runEnableSSH(ctx context.Context, t *terminal.Terminal, s EnableSSHStore, deps enableSSHDeps) error {
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev enable-ssh is only supported on Linux")
	}

	registered, err := deps.registrationStore.Exists()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !registered {
		return fmt.Errorf("no registration found; this machine does not appear to be registered\nRun 'brev register' to register your device first")
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return fmt.Errorf("failed to read registration file: %w", err)
	}

	brevUser, err := s.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return enableSSH(ctx, t, deps, s, reg, brevUser)
}

// enableSSH grants SSH access to the given node for the current Brev user.
// It ensures the managed sshd is installed before granting access.
func enableSSH(
	ctx context.Context,
	t *terminal.Terminal,
	deps enableSSHDeps,
	tokenProvider externalnode.TokenProvider,
	reg *register.DeviceRegistration,
	brevUser *entity.User,
) error {
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green("Enabling SSH access on this device"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Brev user:  %s\n", brevUser.ID)
	t.Vprintf("  Linux user: %s\n", u.Username)
	t.Vprint("")

	if !brevSSHDRunning() {
		t.Vprint("This will install and configure a managed SSH daemon on port 2222.")
		t.Vprint("It will:")
		t.Vprint("  - Install/upgrade openssh-server if needed")
		t.Vprint("  - Create a dedicated sshd config at /etc/brev-sshd/sshd_config")
		t.Vprint("  - Generate a dedicated ed25519 host key")
		t.Vprint("  - Start a systemd service (brev-sshd) listening on port 2222")
		t.Vprint("")
		t.Vprint("Setting up managed SSH daemon...")
		if err := deps.sshd.Install(); err != nil {
			return fmt.Errorf("managed sshd setup failed: %w", err)
		}
		t.Vprint(t.Green("  Managed SSH daemon ready (port 2222)."))
		t.Vprint("")
	}

	if err := register.GrantSSHAccessToNode(ctx, t, deps.nodeClients, tokenProvider, reg, brevUser, u); err != nil {
		return fmt.Errorf("enable SSH failed: %w", err)
	}

	t.Vprint(t.Green(fmt.Sprintf("SSH access enabled. You can now SSH to this device via: brev shell %s", reg.DisplayName)))
	return nil
}

// brevSSHDRunning returns true if the brev-sshd systemd service is active.
func brevSSHDRunning() bool {
	out, err := exec.Command("systemctl", "is-active", "brev-sshd").Output() //nolint:gosec // fixed service name
	return err == nil && strings.TrimSpace(string(out)) == "active"
}
