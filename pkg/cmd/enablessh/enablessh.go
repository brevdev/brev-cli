// Package enablessh implements brev enable-ssh.
package enablessh

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
	breverrors "github.com/brevdev/brev-cli/pkg/errors"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/sshcert"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type EnableSSHStore interface {
	GetCurrentUser() (*entity.User, error)
	GetAccessToken() (string, error)
}

type enableSSHDeps struct {
	platform          externalnode.PlatformChecker
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	prompter          register.SSHAccessPrompter
	// currentUser resolves the OS user for authorized_keys operations.
	currentUser func() (*user.User, error)
	lookupUser  func(string) (*user.User, error)
}

func defaultEnableSSHDeps() enableSSHDeps {
	return enableSSHDeps{
		platform:          register.LinuxPlatform{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
		prompter:          register.TerminalPrompter{},
		currentUser:       user.Current,
		lookupUser:        user.Lookup,
	}
}

func NewCmdEnableSSH(t *terminal.Terminal, store EnableSSHStore) *cobra.Command {
	var linuxUserFlag string
	var sshPortFlag int32

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "enable-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Trust the Brev certificate authority on this device for SSH",
		Long:                  "Writes the Brev certificate authority to authorized_keys, allowing this device to be an SSH target. Interactive mode prompts for the Linux user and SSH port. Non-interactive mode requires both --linux-user and --ssh-port. Users are granted access with 'brev grant-ssh'.",
		Example:               "  # Interactive\n  brev enable-ssh\n\n  # Non-interactive\n  brev enable-ssh --linux-user ubuntu --ssh-port 22",
		RunE: func(cmd *cobra.Command, args []string) error {
			interactive := !cmd.Flags().Changed("linux-user") && !cmd.Flags().Changed("ssh-port")
			return runEnableSSH(cmd.Context(), t, store, defaultEnableSSHDeps(), enableSSHOpts{
				interactive:   interactive,
				linuxUsername: linuxUserFlag,
				sshPort:       sshPortFlag,
			})
		},
	}
	cmd.Flags().StringVar(&linuxUserFlag, "linux-user", "", "Linux username to enable SSH for (required in non-interactive mode)")
	cmd.Flags().Int32Var(&sshPortFlag, "ssh-port", 0, "SSH destination port (required in non-interactive mode)")

	return cmd
}

type enableSSHOpts struct {
	interactive   bool
	linuxUsername string
	sshPort       int32
}

func runEnableSSH(ctx context.Context, t *terminal.Terminal, s EnableSSHStore, deps enableSSHDeps, opts enableSSHOpts) error {
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev enable-ssh is only supported on Linux")
	}
	if err := validateEnableSSHOpts(opts); err != nil {
		return err
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return fmt.Errorf("failed to read registration file: %w", err)
	}

	return enableSSH(ctx, t, deps, s, reg, opts)
}

func enableSSH(
	ctx context.Context,
	t *terminal.Terminal,
	deps enableSSHDeps,
	s EnableSSHStore,
	reg *register.DeviceRegistration,
	opts enableSSHOpts,
) error {
	linuxUser, err := resolveLinuxUser(t, deps, opts)
	if err != nil {
		return err
	}
	linuxUsername := linuxUser.Username

	checkSSHDaemon(t)

	t.Vprint("")
	t.Vprint(t.Green("Enabling SSH on this device"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Linux user: %s\n", linuxUsername)
	t.Vprint("")

	caPublicKey := reg.CertificateAuthority
	if caPublicKey == "" {
		node, err := fetchRegisteredNode(ctx, deps, s, reg)
		if err != nil {
			return fmt.Errorf("enable SSH failed: %w", err)
		}
		if node.GetLabels()[sshcert.LabelKeySSHProvider] != sshcert.SSHProviderCertAuth {
			return legacyEnableSSH(ctx, t, deps, s, reg, node, linuxUsername, opts)
		}
		caPublicKey = node.GetCertificateAuthority()
	}

	if err := installCertAuthority(linuxUser, caPublicKey, reg.ExternalNodeID, linuxUsername); err != nil {
		return fmt.Errorf("enable SSH failed: %w", err)
	}
	t.Vprint(t.Green("  Certificate authority written to authorized_keys."))

	if err := ensureSSHPort(ctx, t, deps, s, reg, opts); err != nil {
		return fmt.Errorf("enable SSH failed: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green("SSH enabled on this device. No one has SSH access yet — grant it with: brev grant-ssh"))
	return nil
}

func ensureSSHPort(ctx context.Context, t *terminal.Terminal, deps enableSSHDeps, s EnableSSHStore, reg *register.DeviceRegistration, opts enableSSHOpts) error {
	sshPort := opts.sshPort
	if opts.interactive {
		var err error
		sshPort, err = register.PromptSSHPort(t, deps.prompter)
		if err != nil {
			return fmt.Errorf("reading SSH port: %w", err)
		}
	}

	ports, err := fetchRegisteredNode(ctx, deps, s, reg)
	if err != nil {
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Note: could not check existing ports: %v", err)))
	}

	if p := findExistingSSHPort(ports, sshPort); p != nil {
		t.Vprintf("  SSH port already allocated (%s).\n", register.FormatPortLabel(p))
		return nil
	}

	if _, err := register.OpenSSHPort(ctx, t, deps.nodeClients, s, reg, sshPort); err != nil {
		// A prior invocation or concurrent request may have allocated the port
		// even when OpenPort returns an error. Confirm the resulting state before
		// deciding whether the operation failed.
		refreshed, refreshErr := fetchRegisteredNode(ctx, deps, s, reg)
		if refreshErr == nil {
			if p := findExistingSSHPort(refreshed, sshPort); p != nil {
				t.Vprintf("  SSH port already allocated (%s).\n", register.FormatPortLabel(p))
				return nil
			}
		}
		if isPortAlreadyAllocatedError(err, sshPort) {
			t.Vprintf("  SSH port %d already allocated.\n", sshPort)
			return nil
		}
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func findExistingSSHPort(node *nodev1.ExternalNode, destinationPort int32) *nodev1.Port {
	for _, p := range node.GetPorts() {
		if p.GetServerPort() == destinationPort || (p.GetServerPort() == 0 && p.GetPortNumber() == destinationPort) {
			return p
		}
	}
	return nil
}

func isPortAlreadyAllocatedError(err error, port int32) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), fmt.Sprintf("port %d is already allocated", port))
}

func validateEnableSSHOpts(opts enableSSHOpts) error {
	if opts.interactive {
		return nil
	}
	if strings.TrimSpace(opts.linuxUsername) == "" || opts.sshPort == 0 {
		return fmt.Errorf("in non-interactive mode --linux-user and --ssh-port are required")
	}
	if opts.sshPort < 1 || opts.sshPort > 65535 {
		return fmt.Errorf("invalid --ssh-port %d: port must be between 1 and 65535", opts.sshPort)
	}
	return nil
}

func resolveLinuxUser(t *terminal.Terminal, deps enableSSHDeps, opts enableSSHOpts) (*user.User, error) {
	if opts.interactive {
		return promptLinuxUser(t, deps)
	}
	linuxUsername := strings.TrimSpace(opts.linuxUsername)
	linuxUser, err := deps.lookupUser(linuxUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to find Linux user %q: %w", linuxUsername, err)
	}
	return linuxUser, nil
}

func promptLinuxUser(t *terminal.Terminal, deps enableSSHDeps) (*user.User, error) {
	currentLinuxUser, err := deps.currentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to determine current Linux user: %w", err)
	}
	linuxUsername, err := register.PromptLinuxUsername(t, deps.prompter, currentLinuxUser.Username)
	if err != nil {
		return nil, fmt.Errorf("reading Linux username: %w", err)
	}
	linuxUsername = strings.TrimSpace(linuxUsername)
	if linuxUsername == currentLinuxUser.Username {
		return currentLinuxUser, nil
	}

	linuxUser, err := deps.lookupUser(linuxUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to find Linux user %q: %w", linuxUsername, err)
	}
	return linuxUser, nil
}

func legacyEnableSSH(
	ctx context.Context,
	t *terminal.Terminal,
	deps enableSSHDeps,
	s EnableSSHStore,
	reg *register.DeviceRegistration,
	node *nodev1.ExternalNode,
	linuxUsername string,
	opts enableSSHOpts,
) error {
	brevUser, err := s.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("enable SSH failed: %w", err)
	}

	brevPortID, err := resolveLegacySSHPort(ctx, t, deps, s, reg, node, opts)
	if err != nil {
		return fmt.Errorf("enable SSH failed: %w", err)
	}

	if err := register.SetupAndRegisterNodeSSHAccess(ctx, t, deps.nodeClients, s, reg, brevUser, linuxUsername, brevPortID); err != nil {
		return fmt.Errorf("enable SSH failed: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green(fmt.Sprintf("SSH access enabled. You can now SSH to this device via: brev shell %s", reg.DisplayName)))
	return nil
}

func resolveLegacySSHPort(
	ctx context.Context,
	t *terminal.Terminal,
	deps enableSSHDeps,
	s EnableSSHStore,
	reg *register.DeviceRegistration,
	node *nodev1.ExternalNode,
	opts enableSSHOpts,
) (string, error) {
	if opts.interactive {
		portID, err := register.ResolveSSHAccessPort(ctx, t, deps.prompter, deps.nodeClients, s, reg, node)
		if err != nil {
			return "", fmt.Errorf("resolve SSH access port: %w", err)
		}
		return portID, nil
	}
	if p := findExistingSSHPort(node, opts.sshPort); p != nil {
		t.Vprintf("  SSH port already allocated (%s).\n", register.FormatPortLabel(p))
		return p.GetPortId(), nil
	}

	portID, err := register.OpenSSHPort(ctx, t, deps.nodeClients, s, reg, opts.sshPort)
	if err == nil {
		return portID, nil
	}
	refreshed, refreshErr := fetchRegisteredNode(ctx, deps, s, reg)
	if refreshErr == nil {
		if p := findExistingSSHPort(refreshed, opts.sshPort); p != nil {
			t.Vprintf("  SSH port already allocated (%s).\n", register.FormatPortLabel(p))
			return p.GetPortId(), nil
		}
	}
	return "", fmt.Errorf("open SSH port: %w", err)
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
	deps enableSSHDeps,
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
