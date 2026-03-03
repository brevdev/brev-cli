package register

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

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

	taggedKey := pubKey + " " + BrevKeyComment

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
