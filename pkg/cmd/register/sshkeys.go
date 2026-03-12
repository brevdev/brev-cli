package register

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/cenkalti/backoff/v4"
)

// SelectNodeFromList prompts the user to select a node from the list, then fetches and returns the full node.
// When this machine is registered, that node is marked " — this node" in the list.
// Call this after ListNodes when in interactive mode (same pattern as SelectOrganizationInteractive).
func SelectNodeFromList(ctx context.Context, t *terminal.Terminal, prompter terminal.Selector, registrationStore RegistrationStore, nodes []*nodev1.ExternalNode) (*nodev1.ExternalNode, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes found in organization")
	}
	var thisNodeID string
	if reg, err := registrationStore.Load(); err == nil && reg != nil {
		thisNodeID = reg.ExternalNodeID
	}
	t.Vprint("")
	thisIdx := -1
	labels := make([]string, len(nodes))
	for i, n := range nodes {
		if thisNodeID != "" && n.GetExternalNodeId() == thisNodeID {
			labels[i] = fmt.Sprintf("%s (%s) — this node", n.GetName(), n.GetExternalNodeId())
			thisIdx = i
		} else {
			labels[i] = fmt.Sprintf("%s (%s)", n.GetName(), n.GetExternalNodeId())
		}
	}
	if thisIdx > 0 {
		nodes[0], nodes[thisIdx] = nodes[thisIdx], nodes[0]
		labels[0], labels[thisIdx] = labels[thisIdx], labels[0]
	}
	chosen := prompter.Select("Select node", labels)
	var selected *nodev1.ExternalNode
	for i, label := range labels {
		if label == chosen {
			selected = nodes[i]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("selected item did not match any node")
	}
	return selected, nil
}

const (
	backoffInitialInterval = 1 * time.Second
	backoffMaxInterval     = 10 * time.Second
	backoffMaxElapsedTime  = 1 * time.Minute

	backoffPrintRound = 500 * time.Millisecond
)

// BrevKeyPrefix is the marker prefix appended to every SSH key that Brev
// installs. Both old-format ("# brev-cli") and new-format
// ("# brev-cli user_id=xxx") keys start with this prefix.
const BrevKeyPrefix = "# brev-cli"

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

// OpenSSHPort calls the OpenPort RPC to allocate an SSH port on the node.
// This must be called before GrantSSHAccessToNode when enabling SSH for the
// first time on a device. The call is idempotent — if the port is already
// open, the server returns the existing allocation.
func OpenSSHPort(
	ctx context.Context,
	t *terminal.Terminal,
	nodeClients externalnode.NodeClientFactory,
	tokenProvider externalnode.TokenProvider,
	reg *DeviceRegistration,
	port int32,
) error {
	client := nodeClients.NewNodeClient(tokenProvider, config.GlobalConfig.GetBrevPublicAPIURL())
	_, err := client.OpenPort(ctx, connect.NewRequest(&nodev1.OpenPortRequest{
		ExternalNodeId: reg.ExternalNodeID,
		Protocol:       nodev1.PortProtocol_PORT_PROTOCOL_SSH,
		PortNumber:     port,
	}))
	if err != nil {
		return fmt.Errorf("failed to allocate SSH port: %w", err)
	}
	t.Vprintf("  SSH port %d allocated.\n", port)
	return nil
}

// SetupAndRegisterNodeSSHAccess installs the user's public key in authorized_keys and
// calls GrantNodeSSHAccess to record access server-side. If the RPC fails,
// the installed key is rolled back. osUser is the target username on the
// remote device (e.g. "ubuntu"), used both in the RPC and to locate the
// correct ~/.ssh/authorized_keys for local key installation.
func SetupAndRegisterNodeSSHAccess(
	ctx context.Context,
	t *terminal.Terminal,
	nodeClients externalnode.NodeClientFactory,
	tokenProvider externalnode.TokenProvider,
	reg *DeviceRegistration,
	targetUser *entity.User,
	linuxUsername string,
) error {
	// Look up the local OS user for key installation. This may differ from
	// the caller (e.g. running as root, installing keys for "ubuntu").
	osUser, lookupErr := user.Lookup(linuxUsername)
	if lookupErr != nil {
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: could not look up local user %q — skipping local key installation", linuxUsername)))
	}

	if osUser != nil && targetUser.PublicKey != "" {
		if added, err := InstallAuthorizedKey(osUser, targetUser.PublicKey, targetUser.ID); err != nil {
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to install SSH public key: %v", err)))
		} else if added {
			t.Vprint("  Brev public key added to authorized_keys.")
		}
	}

	client := nodeClients.NewNodeClient(tokenProvider, config.GlobalConfig.GetBrevPublicAPIURL())

	backoffCtx := backoff.WithContext(backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(backoffInitialInterval),
		backoff.WithMaxInterval(backoffMaxInterval),
		backoff.WithMaxElapsedTime(backoffMaxElapsedTime),
	), ctx)

	opToTry := func() error {
		_, err := client.GrantNodeSSHAccess(ctx, connect.NewRequest(&nodev1.GrantNodeSSHAccessRequest{
			ExternalNodeId: reg.ExternalNodeID,
			UserId:         targetUser.ID,
			LinuxUser:      linuxUsername,
		}))
		if err != nil {
			// Retryable error
			var connectErr *connect.Error
			if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeInternal {
				return fmt.Errorf("failed to grant SSH access (transient): %w", err)
			}

			// Permanent error — roll back the key so we don't leave an unrecorded entry and abort the backoff retry
			if osUser != nil && targetUser.PublicKey != "" {
				if rerr := RemoveAuthorizedKey(osUser, targetUser.PublicKey); rerr != nil {
					t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to remove SSH key after failed grant: %v", rerr)))
				}
			}
			return backoff.Permanent(fmt.Errorf("failed to grant SSH access: %w", err))
		}

		return nil
	}
	onOpErr := func(err error, d time.Duration) {
		t.Vprintf("  SSH access not yet granted; retrying in: %s...\n", d.Round(backoffPrintRound))
	}
	err := backoff.RetryNotify(opToTry, backoffCtx, onOpErr)
	if err != nil {
		return fmt.Errorf("failed to grant SSH access: %w", err)
	}
	return nil
}

const defaultSSHPort = 22

// testSSHPort is set by tests to avoid blocking on stdin. When non-nil,
// PromptSSHPort returns this value without prompting.
var testSSHPort *int32

// SetTestSSHPort sets the port returned by PromptSSHPort without prompting.
// Only for use in tests; call ClearTestSSHPort when done.
func SetTestSSHPort(port int32) { testSSHPort = &port }

// ClearTestSSHPort clears the test port override.
func ClearTestSSHPort() { testSSHPort = nil }

// PromptSSHPort prompts the user for the target SSH port, defaulting to 22 if
// they press Enter or leave it empty. Returns an error for invalid port numbers.
// Uses a single print + stdin read to avoid promptui rendering the label twice.
func PromptSSHPort(t *terminal.Terminal) (int32, error) {
	if testSSHPort != nil {
		return *testSSHPort, nil
	}
	t.Vprintf("  %s ", t.Green("SSH port (default 22):"))
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("reading input: %w", err)
	}
	portStr := strings.TrimSpace(line)
	if portStr == "" {
		return defaultSSHPort, nil
	}
	n, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q: %w", portStr, err)
	}
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535, got %d", n)
	}
	return int32(n), nil
}

// InstallAuthorizedKey appends the given public key to the user's
// ~/.ssh/authorized_keys if it isn't already present. The key is tagged with
// a brev-cli comment (including the user ID) so it can be identified and
// removed later by RemoveBrevAuthorizedKeys or ListBrevAuthorizedKeys.
// Returns true if the key was newly written, false if it was already present.
func InstallAuthorizedKey(u *user.User, pubKey string, brevUserID string) (bool, error) {
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

	taggedKey := pubKey + " " + BrevKeyTag(brevUserID)

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
