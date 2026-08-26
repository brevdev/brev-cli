// Package allowssh implements brev allow-ssh.
package allowssh

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/sshcert"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type AllowSSHStore interface {
	GetCurrentUser() (*entity.User, error)
	GetAccessToken() (string, error)
}

type allowSSHDeps struct {
	platform          externalnode.PlatformChecker
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	prompter          terminal.Selector
}

func defaultAllowSSHDeps() allowSSHDeps {
	return allowSSHDeps{
		platform:          register.LinuxPlatform{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
		prompter:          register.TerminalPrompter{},
	}
}

func NewCmdAllowSSH(t *terminal.Terminal, store AllowSSHStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "allow-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Trust the Brev certificate authority on this device for SSH",
		Long:                  "Writes the Brev certificate authority to authorized_keys, allowing this device to be an SSH target for the current Linux user. Users are granted access with 'brev grant-ssh'.",
		Example:               "  brev allow-ssh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAllowSSH(cmd.Context(), t, store, defaultAllowSSHDeps())
		},
	}

	return cmd
}

func runAllowSSH(ctx context.Context, t *terminal.Terminal, s AllowSSHStore, deps allowSSHDeps) error {
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev allow-ssh is only supported on Linux")
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return fmt.Errorf("failed to read registration file: %w", err)
	}

	brevUser, err := s.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	return allowSSH(ctx, t, deps, s, reg, brevUser)
}

func allowSSH(
	ctx context.Context,
	t *terminal.Terminal,
	deps allowSSHDeps,
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
	t.Vprint(t.Green("Allowing SSH on this device"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Linux user: %s\n", linuxUsername)
	t.Vprint("")

	node, err := fetchRegisteredNode(ctx, deps, tokenProvider, reg)
	if err != nil {
		return fmt.Errorf("allow SSH failed: %w", err)
	}

	if node.GetLabels()[sshcert.LabelKeySSHProvider] != sshcert.SSHProviderCertAuth {
		return legacyEnableSSH(ctx, t, deps, tokenProvider, reg, brevUser, node, linuxUsername)
	}

	caPublicKey := node.GetCertificateAuthority()

	if err := installCertAuthority(linuxUser, caPublicKey, reg.ExternalNodeID, linuxUsername); err != nil {
		return fmt.Errorf("allow SSH failed: %w", err)
	}
	t.Vprint(t.Green("  Certificate authority written to authorized_keys."))

	t.Vprint("")
	t.Vprint(t.Green("SSH allowed on this device. No one has SSH access yet — grant it with: brev grant-ssh"))
	return nil
}

func legacyEnableSSH(
	ctx context.Context,
	t *terminal.Terminal,
	deps allowSSHDeps,
	tokenProvider externalnode.TokenProvider,
	reg *register.DeviceRegistration,
	brevUser *entity.User,
	node *nodev1.ExternalNode,
	linuxUsername string,
) error {
	brevPortID, err := register.ResolveSSHAccessPort(ctx, t, deps.prompter, deps.nodeClients, tokenProvider, reg, node)
	if err != nil {
		return fmt.Errorf("allow SSH failed: %w", err)
	}

	if err := register.SetupAndRegisterNodeSSHAccess(ctx, t, deps.nodeClients, tokenProvider, reg, brevUser, linuxUsername, brevPortID); err != nil {
		return fmt.Errorf("allow SSH failed: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green(fmt.Sprintf("SSH access enabled. You can now SSH to this device via: brev shell %s", reg.DisplayName)))
	return nil
}

func installCertAuthority(osUser *user.User, caPublicKey, nodeID, linuxUser string) error {
	if caPublicKey == "" {
		return fmt.Errorf("certificate authority public key is required")
	}

	principal := fmt.Sprintf("brev:v1:vm:%s:login:%s", nodeID, linuxUser)
	entry := fmt.Sprintf("cert-authority,principals=\"%s\" %s", principal, strings.TrimSpace(caPublicKey))

	sshDir := filepath.Join(osUser.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("creating .ssh directory: %w", err)
	}

	authKeysPath := filepath.Join(sshDir, "authorized_keys")

	existing, err := os.ReadFile(authKeysPath) // #nosec G304
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading authorized_keys: %w", err)
	}

	// skip if the entry already exists.
	for line := range strings.SplitSeq(string(existing), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}

	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry + "\n"

	if err := os.WriteFile(authKeysPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing authorized_keys: %w", err)
	}

	return nil
}

func fetchRegisteredNode(
	ctx context.Context,
	deps allowSSHDeps,
	tokenProvider externalnode.TokenProvider,
	reg *register.DeviceRegistration,
) (*nodev1.ExternalNode, error) {
	client := deps.nodeClients.NewNodeClient(tokenProvider, config.GlobalConfig.GetBrevPublicAPIURL())
	resp, err := client.GetNode(ctx, connect.NewRequest(&nodev1.GetNodeRequest{
		ExternalNodeId: reg.ExternalNodeID,
	}))
	if err != nil {
		return nil, fmt.Errorf("error retrieving node: %w", err)
	}
	return resp.Msg.GetExternalNode(), nil
}

func checkSSHDaemon(t *terminal.Terminal) {
	for _, svc := range []string{"ssh", "sshd"} {
		out, err := exec.Command("systemctl", "is-active", svc).Output() //nolint:gosec // fixed service names
		if err == nil && len(out) > 0 && string(out[:len(out)-1]) == "active" {
			return
		}
	}
	t.Vprintf("  %s\n", t.Yellow("Warning: SSH daemon does not appear to be running. SSH access may not work until sshd is started."))
}
