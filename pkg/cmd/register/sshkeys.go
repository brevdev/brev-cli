package register

import (
	"context"
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

// BrevKeyPrefix is the marker prefix appended to every SSH key that Brev
// installs. Both old-format ("# brev-cli") and new-format
// ("# brev-cli user_id=xxx") keys start with this prefix.
const BrevKeyPrefix = "# brev-cli"

// BrevKeyComment is kept as an alias for backward compatibility with tests
// and callers that reference the old constant name.
const BrevKeyComment = BrevKeyPrefix

// BrevKeyTag returns a comment tag that encodes the Brev user ID.
// Example: "# brev-cli user_id=user_abc123"
func BrevKeyTag(userID string) string {
	if userID == "" {
		return BrevKeyPrefix
	}
	return fmt.Sprintf("%s user_id=%s", BrevKeyPrefix, userID)
}

// BrevAuthorizedKey represents a single Brev-managed key found in
// authorized_keys.
type BrevAuthorizedKey struct {
	Line       string // full line from authorized_keys
	KeyContent string // the ssh key portion (without the brev comment)
	UserID     string // parsed from "user_id=xxx", empty for old-format keys
}

// ListBrevAuthorizedKeys reads ~/.ssh/authorized_keys and returns all lines
// containing the BrevKeyPrefix marker.
func ListBrevAuthorizedKeys(u *user.User) ([]BrevAuthorizedKey, error) {
	authKeysPath := filepath.Join(u.HomeDir, ".ssh", "authorized_keys")

	data, err := os.ReadFile(authKeysPath) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading authorized_keys: %w", err)
	}

	var keys []BrevAuthorizedKey
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, BrevKeyPrefix) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		bk := BrevAuthorizedKey{Line: trimmed}

		// Split on " # brev-cli" to get the key content before the tag.
		if idx := strings.Index(trimmed, " "+BrevKeyPrefix); idx >= 0 {
			bk.KeyContent = trimmed[:idx]
			tag := trimmed[idx+1:] // the "# brev-cli ..." part
			// Parse user_id if present.
			if uidIdx := strings.Index(tag, "user_id="); uidIdx >= 0 {
				rest := tag[uidIdx+len("user_id="):]
				// user_id value ends at next space or end of string.
				if spIdx := strings.Index(rest, " "); spIdx >= 0 {
					bk.UserID = rest[:spIdx]
				} else {
					bk.UserID = rest
				}
			}
		} else {
			bk.KeyContent = trimmed
		}

		keys = append(keys, bk)
	}

	return keys, nil
}

// RemoveAuthorizedKeyLine removes an exact line from authorized_keys.
func RemoveAuthorizedKeyLine(u *user.User, line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
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
	for _, l := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(l) == line {
			continue
		}
		kept = append(kept, l)
	}

	result := strings.Join(kept, "\n")
	if err := os.WriteFile(authKeysPath, []byte(result), 0o600); err != nil {
		return fmt.Errorf("writing authorized_keys: %w", err)
	}
	return nil
}

// GrantSSHAccessToNode installs the user's public key in authorized_keys and
// calls GrantNodeSSHAccess to record access server-side. If the RPC fails,
// the installed key is rolled back.
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
		if err := InstallAuthorizedKey(osUser, targetUser.PublicKey, targetUser.ID); err != nil {
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to install SSH public key: %v", err)))
		} else {
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
		if targetUser.PublicKey != "" {
			if rerr := RemoveAuthorizedKey(osUser, targetUser.PublicKey); rerr != nil {
				t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to remove SSH key after failed grant: %v", rerr)))
			}
		}
		return fmt.Errorf("failed to grant SSH access: %w", err)
	}

	return nil
}

// RevokeSSHAccessFromNode removes the user's public key from authorized_keys
// and calls RevokeNodeSSHAccess to record the revocation server-side.
// No rollback is performed if the RPC fails — the key stays removed for safety.
func RevokeSSHAccessFromNode(
	ctx context.Context,
	t *terminal.Terminal,
	nodeClients externalnode.NodeClientFactory,
	tokenProvider externalnode.TokenProvider,
	reg *DeviceRegistration,
	targetUser *entity.User,
	osUser *user.User,
) error {
	if targetUser.PublicKey != "" {
		if err := RemoveAuthorizedKey(osUser, targetUser.PublicKey); err != nil {
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to remove SSH public key: %v", err)))
		} else {
			t.Vprint("  Brev public key removed from authorized_keys.")
		}
	}

	client := nodeClients.NewNodeClient(tokenProvider, config.GlobalConfig.GetBrevPublicAPIURL())
	_, err := client.RevokeNodeSSHAccess(ctx, connect.NewRequest(&nodev1.RevokeNodeSSHAccessRequest{
		ExternalNodeId: reg.ExternalNodeID,
		UserId:         targetUser.ID,
		LinuxUser:      osUser.Username,
	}))
	if err != nil {
		return fmt.Errorf("failed to revoke SSH access: %w", err)
	}

	return nil
}

// InstallAuthorizedKey appends the given public key to the user's
// ~/.ssh/authorized_keys if it isn't already present. The key is tagged with
// a brev-cli comment (including the user ID) so it can be identified and
// removed later by RemoveBrevAuthorizedKeys or ListBrevAuthorizedKeys.
func InstallAuthorizedKey(u *user.User, pubKey string, brevUserID string) error {
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

	taggedKey := pubKey + " " + BrevKeyTag(brevUserID)

	if strings.Contains(string(existing), taggedKey) {
		return nil // already present with tag
	}

	// If the key exists but isn't tagged, replace it with the tagged version
	// so that RemoveBrevAuthorizedKeys can find it later.
	if strings.Contains(string(existing), pubKey) {
		updated := strings.ReplaceAll(string(existing), pubKey, taggedKey)
		if err := os.WriteFile(authKeysPath, []byte(updated), 0o600); err != nil {
			return fmt.Errorf("writing authorized_keys: %w", err)
		}
		return nil
	}

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
		if strings.Contains(line, BrevKeyPrefix) {
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
