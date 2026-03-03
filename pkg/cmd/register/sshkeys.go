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
