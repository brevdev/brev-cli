// Package enablessh provides the brev enableSSH command for enabling SSH access
// to a registered external node.
package enablessh

import (
	"context"
	"fmt"
	"os/exec"
	"os/user"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

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
	GetAccessToken() (string, error)
}

type sshAccessProvisioner interface {
	Provision(
		context.Context,
		*terminal.Terminal,
		externalnode.TokenProvider,
		*register.DeviceRegistration,
		*entity.User,
		*nodev1.ExternalNode,
	) error
}

// enableSSHDeps bundles the side-effecting dependencies of runEnableSSH so they
// can be replaced in tests.
type enableSSHDeps struct {
	platform          externalnode.PlatformChecker
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	tunnel            register.NetBirdConnector
	provisioner       sshAccessProvisioner
}

type defaultSSHAccessProvisioner struct {
	prompter    terminal.Selector
	nodeClients externalnode.NodeClientFactory
}

func defaultEnableSSHDeps() enableSSHDeps {
	prompter := register.TerminalPrompter{}
	nodeClients := register.DefaultNodeClientFactory{}
	return enableSSHDeps{
		platform:          register.LinuxPlatform{},
		nodeClients:       nodeClients,
		registrationStore: register.NewFileRegistrationStore(),
		tunnel:            register.Netbird{},
		provisioner: defaultSSHAccessProvisioner{
			prompter:    prompter,
			nodeClients: nodeClients,
		},
	}
}

func NewCmdEnableSSH(t *terminal.Terminal, store EnableSSHStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "enable-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Enable SSH access to this joined node",
		Long:                  "Enable SSH access to this joined node for the current Brev user.",
		Example:               "  brev enable-ssh",
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnableSSH(cmd.Context(), t, store, defaultEnableSSHDeps())
		},
	}

	return cmd
}

func runEnableSSH(ctx context.Context, t *terminal.Terminal, s EnableSSHStore, deps enableSSHDeps) error {
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev enable-ssh is only supported on Linux")
	}

	exists, err := deps.registrationStore.Exists()
	if err != nil {
		return fmt.Errorf("check joined-device registration: %w", err)
	}
	if !exists {
		return breverrors.New(`This machine has not joined a Brev network; run "brev join" first.`)
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return fmt.Errorf("read joined-device registration: %w", err)
	}

	brevUser, err := s.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	node, err := register.FetchRegisteredNode(ctx, deps.nodeClients, s, reg)
	if err != nil {
		return fmt.Errorf("enable SSH failed: %w", err)
	}
	if err := deps.tunnel.EnsureConnected(ctx); err != nil {
		return fmt.Errorf("enable SSH requires a connected Brev tunnel: %w", err)
	}
	if err := deps.provisioner.Provision(ctx, t, s, reg, brevUser, node); err != nil {
		return fmt.Errorf("enable SSH failed: %w", err)
	}

	t.Vprint(t.Green(fmt.Sprintf("SSH access enabled. You can now SSH to this device via: brev shell %s", reg.DisplayName)))
	return nil
}

// Provision grants SSH access to the joined node for the current Brev user.
// This is the "reflexive grant" — granting yourself SSH access to the device.
func (p defaultSSHAccessProvisioner) Provision(
	ctx context.Context,
	t *terminal.Terminal,
	tokenProvider externalnode.TokenProvider,
	reg *register.DeviceRegistration,
	brevUser *entity.User,
	node *nodev1.ExternalNode,
) error {
	linuxUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}
	linuxUsername := linuxUser.Username

	checkSSHDaemon(t)

	t.Vprint("")
	t.Vprint(t.Green("Enabling SSH access on this device"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Brev user:  %s\n", brevUser.ID)
	t.Vprintf("  Linux user: %s\n", linuxUsername)
	t.Vprint("")

	brevPortID, err := register.ResolveSSHAccessPort(ctx, t, p.prompter, p.nodeClients, tokenProvider, reg, node)
	if err != nil {
		return err
	}

	if err := register.SetupAndRegisterNodeSSHAccess(ctx, t, p.nodeClients, tokenProvider, reg, brevUser, linuxUsername, brevPortID); err != nil {
		return err
	}

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
