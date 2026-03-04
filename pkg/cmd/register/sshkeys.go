package register

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

// BrevKeyComment is the marker appended to every SSH key that Brev installs.
// It allows RemoveBrevAuthorizedKeys to identify and remove exactly those keys.
const BrevKeyComment = "# brev-cli"

// GrantSSHAccessToNode installs the user's public key in authorized_keys and
// calls GrantNodeSSHAccess to record access server-side.
//
// On a transient transport error (connect.CodeInternal, e.g. connection reset),
// the key is left in authorized_keys so the caller can retry without
// re-installing it. On a permanent application error (auth, not found, etc.)
// the key is rolled back.
func GrantSSHAccessToNode(
	ctx context.Context,
	t *terminal.Terminal,
	nodeClients externalnode.NodeClientFactory,
	tokenProvider externalnode.TokenProvider,
	reg *DeviceRegistration,
	targetUser *entity.User,
	osUser *user.User,
) error {
	if targetUser.PublicKey != "" {
		added, err := InstallAuthorizedKey(osUser, targetUser.PublicKey)
		if err != nil {
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to install SSH public key: %v", err)))
		} else if added {
			t.Vprint("  Brev public key added to authorized_keys.")
		}
	}

	client := nodeClients.NewNodeClient(tokenProvider, config.GlobalConfig.GetBrevPublicAPIURL())
	_, err := client.GrantNodeSSHAccess(ctx, connect.NewRequest(&nodev1.GrantNodeSSHAccessRequest{
		ExternalNodeId: reg.ExternalNodeID,
		UserId:         targetUser.ID,
		LinuxUser:      osUser.Username,
	}))
	if err != nil {
		// Transport errors (connection reset, EOF) are transient — leave the key
		// installed so retries don't need to reinstall it, and signal the caller
		// with a distinct error type.
		var connectErr *connect.Error
		if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeInternal {
			return fmt.Errorf("failed to grant SSH access (transient): %w", err)
		}
		// Permanent error — roll back the key so we don't leave an unrecorded entry.
		if targetUser.PublicKey != "" {
			if rerr := RemoveAuthorizedKey(osUser, targetUser.PublicKey); rerr != nil {
				t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to remove SSH key after failed grant: %v", rerr)))
			}
		}
		return fmt.Errorf("failed to grant SSH access: %w", err)
	}

	return nil
}

// InstallAuthorizedKey appends the given public key to the user's
// ~/.ssh/authorized_keys if it isn't already present. The key is tagged with
// a brev-cli comment so it can be removed later by RemoveBrevAuthorizedKeys.
// Returns true if the key was newly written, false if it was already present.
func InstallAuthorizedKey(u *user.User, pubKey string) (bool, error) {
	pubKey = strings.TrimSpace(pubKey)
	if pubKey == "" {
		return false, nil
	}

	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return false, fmt.Errorf("creating .ssh directory: %w", err)
	}

	authKeysPath := filepath.Join(sshDir, "authorized_keys")

	existing, err := os.ReadFile(authKeysPath) // #nosec G304
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("reading authorized_keys: %w", err)
	}

	taggedKey := pubKey + " " + BrevKeyComment

	if strings.Contains(string(existing), taggedKey) {
		return false, nil // already present with tag
	}

	// If the key exists but isn't tagged, replace it with the tagged version
	// so that RemoveBrevAuthorizedKeys can find it later.
	if strings.Contains(string(existing), pubKey) {
		updated := strings.ReplaceAll(string(existing), pubKey, taggedKey)
		if err := os.WriteFile(authKeysPath, []byte(updated), 0o600); err != nil {
			return false, fmt.Errorf("writing authorized_keys: %w", err)
		}
		return true, nil
	}

	// Ensure existing content ends with a newline before appending.
	content := string(existing)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += taggedKey + "\n"

	if err := os.WriteFile(authKeysPath, []byte(content), 0o600); err != nil {
		return false, fmt.Errorf("writing authorized_keys: %w", err)
	}

	return true, nil
}

// RemoveAuthorizedKey removes a specific public key from the user's
// ~/.ssh/authorized_keys. It matches the key content regardless of whether
// the brev-cli comment tag is present.
func RemoveAuthorizedKey(u *user.User, pubKey string) error {
	pubKey = strings.TrimSpace(pubKey)
	if pubKey == "" {
		return nil
	}

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
		if strings.Contains(line, pubKey) {
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

// RemoveBrevAuthorizedKeys removes all SSH keys tagged with the brev-cli
// comment from the user's ~/.ssh/authorized_keys. It returns the lines that
// were removed so callers can report what was cleaned up.
func RemoveBrevAuthorizedKeys(u *user.User) ([]string, error) {
	authKeysPath := filepath.Join(u.HomeDir, ".ssh", "authorized_keys")

	existing, err := os.ReadFile(authKeysPath) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading authorized_keys: %w", err)
	}

	var kept []string
	var removed []string
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.Contains(line, BrevKeyComment) {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				removed = append(removed, trimmed)
			}
			continue
		}
		kept = append(kept, line)
	}

	result := strings.Join(kept, "\n")
	if err := os.WriteFile(authKeysPath, []byte(result), 0o600); err != nil {
		return nil, fmt.Errorf("writing authorized_keys: %w", err)
	}
	return removed, nil
}
