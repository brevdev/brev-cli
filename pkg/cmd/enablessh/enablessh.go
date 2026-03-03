// Package enablessh provides the brev enableSSH command for enabling SSH access
// to a registered external node.
package enablessh

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

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
		if err := InstallAuthorizedKey(u, brevUser.PublicKey); err != nil {
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

// BrevKeyComment is the marker appended to every SSH key that Brev installs.
// It allows RemoveBrevAuthorizedKeys to identify and remove exactly those keys.
const BrevKeyComment = "# brev-cli"

// InstallAuthorizedKey appends the given public key to the user's
// ~/.ssh/authorized_keys if it isn't already present. The key is tagged with
// a brev-cli comment so it can be removed later by RemoveBrevAuthorizedKeys.
func InstallAuthorizedKey(u *user.User, pubKey string) error {
	pubKey = strings.TrimSpace(pubKey)
	if pubKey == "" {
		return nil
	}

	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("creating .ssh directory: %w", err)
	}

	authKeysPath := filepath.Join(sshDir, "authorized_keys")

	existing, err := os.ReadFile(authKeysPath) // #nosec G304
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading authorized_keys: %w", err)
	}

	if strings.Contains(string(existing), pubKey) {
		return nil // already present (tagged or not)
	}

	taggedKey := pubKey + " " + BrevKeyComment

	// Ensure existing content ends with a newline before appending.
	content := string(existing)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += taggedKey + "\n"

	if err := os.WriteFile(authKeysPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing authorized_keys: %w", err)
	}

	return nil
}

// RemoveBrevAuthorizedKeys removes all SSH keys tagged with the brev-cli
// comment from the user's ~/.ssh/authorized_keys.
func RemoveBrevAuthorizedKeys(u *user.User) error {
	authKeysPath := filepath.Join(u.HomeDir, ".ssh", "authorized_keys")

	existing, err := os.ReadFile(authKeysPath) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading authorized_keys: %w", err)
	}

	var kept []string
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.Contains(line, BrevKeyComment) {
			continue
		}
		kept = append(kept, line)
	}

	result := strings.Join(kept, "\n")
	if err := os.WriteFile(authKeysPath, []byte(result), 0o600); err != nil {
		return fmt.Errorf("writing authorized_keys: %w", err)
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
