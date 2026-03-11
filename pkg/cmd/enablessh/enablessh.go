// Package enablessh provides the brev enableSSH command for enabling SSH access
// to a registered external node.
package enablessh

import (
	"context"
	"fmt"
	"os/exec"
	"os/user"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
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

// enableSSHDeps bundles the side-effecting dependencies of runEnableSSH so they
// can be replaced in tests.
type enableSSHDeps struct {
	platform          externalnode.PlatformChecker
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
}

func defaultEnableSSHDeps() enableSSHDeps {
	return enableSSHDeps{
		platform:          register.LinuxPlatform{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
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
			return runEnableSSH(cmd.Context(), t, store, defaultEnableSSHDeps())
		},
	}

	return cmd
}

func runEnableSSH(ctx context.Context, t *terminal.Terminal, s EnableSSHStore, deps enableSSHDeps) error {
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev enable-ssh is only supported on Linux")
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
// This is the "reflexive grant" — granting yourself SSH access to the device.
func enableSSH(
	ctx context.Context,
	t *terminal.Terminal,
	deps enableSSHDeps,
	tokenProvider externalnode.TokenProvider,
	reg *register.DeviceRegistration,
	brevUser *entity.User,
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

	// Check if the node already has an SSH port allocated (e.g. for another linux user)
	port, err := existingSSHPort(ctx, deps, tokenProvider, reg)
	if err != nil {
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: could not check for existing ports: %v", err)))
	}

	if port != 0 {
		t.Vprintf("  Using existing SSH port %d.\n", port)
	} else {
		t.Vprint("")
		port, err = register.PromptSSHPort(t)
		if err != nil {
			return fmt.Errorf("SSH port: %w", err)
		}

		if err := register.OpenSSHPort(ctx, t, deps.nodeClients, tokenProvider, reg, port); err != nil {
			return fmt.Errorf("enable SSH failed: %w", err)
		}
	}

	if err := register.SetupAndRegisterNodeSSHAccess(ctx, t, deps.nodeClients, tokenProvider, reg, brevUser, linuxUsername); err != nil {
		return fmt.Errorf("enable SSH failed: %w", err)
	}

	t.Vprint(t.Green(fmt.Sprintf("SSH access enabled. You can now SSH to this device via: brev shell %s", reg.DisplayName)))
	return nil
}

// existingSSHPort calls GetNode and returns the PortNumber of an already-allocated
// SSH port, or 0 if none exists
func existingSSHPort(ctx context.Context, deps enableSSHDeps, tokenProvider externalnode.TokenProvider, reg *register.DeviceRegistration) (int32, error) {
	client := deps.nodeClients.NewNodeClient(tokenProvider, config.GlobalConfig.GetBrevPublicAPIURL())
	resp, err := client.GetNode(ctx, connect.NewRequest(&nodev1.GetNodeRequest{
		ExternalNodeId: reg.ExternalNodeID,
		OrganizationId: reg.OrgID,
	}))
	if err != nil {
		return 0, fmt.Errorf("error retrieving node: %w", err)
	}

	for _, p := range resp.Msg.GetExternalNode().GetPorts() {
		// TODO if we ever allow more than one SSH port, this should be modified
		if p.GetProtocol() == nodev1.PortProtocol_PORT_PROTOCOL_SSH {
			return p.GetPortNumber(), nil
		}
	}
	return 0, nil
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
