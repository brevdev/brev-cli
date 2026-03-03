// Package enablessh provides the brev enableSSH command for enabling SSH access
// to a registered external node.
package enablessh

import (
	"context"
	"fmt"
	"os/exec"
	"os/user"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// EnableSSHStore defines the store methods needed by the enableSSH command.
type EnableSSHStore interface {
	GetCurrentUser() (*entity.User, error)
	GetBrevHomePath() (string, error)
	GetAccessToken() (string, error)
}

// PlatformChecker checks whether the current platform is supported.
type PlatformChecker interface {
	IsCompatible() bool
}

// NodeClientFactory creates ConnectRPC ExternalNodeService clients.
type NodeClientFactory interface {
	NewNodeClient(provider register.TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient
}

// enableSSHDeps bundles the side-effecting dependencies of runEnableSSH so they
// can be replaced in tests.
type enableSSHDeps struct {
	platform          PlatformChecker
	nodeClients       NodeClientFactory
	registrationStore register.RegistrationStore
}

func defaultEnableSSHDeps(brevHome string) enableSSHDeps {
	return enableSSHDeps{
		platform:          register.LinuxPlatform{},
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

	return EnableSSH(ctx, t, deps.nodeClients, s, reg, brevUser)
}

// EnableSSH grants SSH access to the given node for the specified Brev user.
// It is exported so that the register command can reuse it after registration.
func EnableSSH(
	ctx context.Context,
	t *terminal.Terminal,
	nodeClients NodeClientFactory,
	tokenProvider register.TokenProvider,
	reg *register.DeviceRegistration,
	brevUser *entity.User,
) error {
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}
	linuxUser := u.Username

	checkSSHDaemon(t)

	t.Vprint("")
	t.Vprint(t.Green("Enabling SSH access on this device"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Brev user:  %s\n", brevUser.ID)
	t.Vprintf("  Linux user: %s\n", linuxUser)
	t.Vprint("")

	if brevUser.PublicKey != "" {
		if err := register.InstallAuthorizedKey(u, brevUser.PublicKey); err != nil {
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to install SSH public key: %v", err)))
		} else {
			t.Vprint("  Brev public key added to authorized_keys.")
		}
	}

	client := nodeClients.NewNodeClient(tokenProvider, config.GlobalConfig.GetBrevPublicAPIURL())
	if _, err := client.GrantNodeSSHAccess(ctx, connect.NewRequest(&nodev1.GrantNodeSSHAccessRequest{
		ExternalNodeId: reg.ExternalNodeID,
		UserId:         brevUser.ID,
		LinuxUser:      linuxUser,
	})); err != nil {
		return fmt.Errorf("failed to enable SSH access: %w", err)
	}

	t.Vprint(t.Green(fmt.Sprintf("SSH access enabled. You can now SSH to this device via: brev shell %s", reg.DisplayName)))
	return nil
}

// checkSSHDaemon prints a warning if neither "ssh" nor "sshd" systemd services
// appear to be active. It never returns an error — it is best-effort.
func checkSSHDaemon(t *terminal.Terminal) {
	for _, svc := range []string{"ssh", "sshd"} {
		out, err := exec.Command("systemctl", "is-active", svc).Output() //nolint:gosec // fixed service names
		if err == nil && len(out) > 0 && string(out[:len(out)-1]) == "active" {
			return
		}
	}
	t.Vprintf("  %s\n", t.Yellow("Warning: SSH daemon does not appear to be running. SSH access may not work until sshd is started."))
}
