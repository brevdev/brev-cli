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
	labels := make([]string, len(nodes))
	for i, n := range nodes {
		labels[i] = fmt.Sprintf("%s (%s)", n.GetName(), n.GetExternalNodeId())
		if thisNodeID != "" && n.GetExternalNodeId() == thisNodeID {
			labels[i] = fmt.Sprintf("%s (%s) — this node", n.GetName(), n.GetExternalNodeId())
		}
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


func SelectPortFromList(_ context.Context, t *terminal.Terminal, prompter terminal.Selector, ports []*nodev1.Port) (*nodev1.Port, error) {
	if len(ports) == 0 {
		return nil, fmt.Errorf("no ports to select")
	}
	if len(ports) == 1 {
		return ports[0], nil
	}
	t.Vprint("")
	labels := make([]string, len(ports))
	for i, p := range ports {
		labels[i] = FormatPortLabel(p)
	}
	chosen := prompter.Select("Select port", labels)
	for i, label := range labels {
		if label == chosen {
			return ports[i], nil
		}
	}
	return nil, fmt.Errorf("selected item did not match any port")
}

const (
	// PortChoiceUseExisting is the interactive option to grant SSH on an allocated port.
	PortChoiceUseExisting = "Use an existing port"
	// PortChoiceOpenNew is the interactive option to open a new port before granting SSH.
	PortChoiceOpenNew = "Open a new port"
)

// ResolveSSHAccessPort prompts for an existing or new port and returns its Brev port ID.
func ResolveSSHAccessPort(
	ctx context.Context,
	t *terminal.Terminal,
	prompter terminal.Selector,
	nodeClients externalnode.NodeClientFactory,
	tokenProvider externalnode.TokenProvider,
	reg *DeviceRegistration,
	node *nodev1.ExternalNode,
) (string, error) {
	ports := node.GetPorts()
	if len(ports) == 0 {
		return openPortForSSHAccess(ctx, t, nodeClients, tokenProvider, reg)
	}

	t.Vprint("")
	choice := prompter.Select("SSH port", []string{PortChoiceUseExisting, PortChoiceOpenNew})
	switch choice {
	case PortChoiceUseExisting:
		selected, err := SelectPortFromList(ctx, t, prompter, ports)
		if err != nil {
			return "", err
		}
		t.Vprintf("  Using port %s.\n", FormatPortLabel(selected))
		return selected.GetPortId(), nil
	case PortChoiceOpenNew:
		return openPortForSSHAccess(ctx, t, nodeClients, tokenProvider, reg)
	default:
		return "", fmt.Errorf("invalid port choice %q", choice)
	}
}

// FormatPortLabel formats a port as "nodePort->connectPort" (e.g. "11640->22").
func FormatPortLabel(p *nodev1.Port) string {
	if p == nil {
		return ""
	}
	if p.GetServerPort() != 0 {
		return fmt.Sprintf("%d->%d", p.GetPortNumber(), p.GetServerPort())
	}
	return fmt.Sprintf("%d", p.GetPortNumber())
}

const (
	backoffInitialInterval = 1 * time.Second
	backoffMaxInterval     = 10 * time.Second
	backoffMaxElapsedTime  = 1 * time.Minute

	backoffPrintRound = 500 * time.Millisecond
)

// BrevKeyPrefixLegacy marks keys written by older CLI versions (# brev-cli).
const BrevKeyPrefixLegacy = "# brev-cli"

// BrevKeyPrefix is an alias for BrevKeyPrefixLegacy (tests and migration).
const BrevKeyPrefix = BrevKeyPrefixLegacy

const (
	brevPortIDField = "brev-portID:"
	brevUserIDField = "brev-userID:"
)

// DevplaneAuthorizedKeysComment is the suffix on Brev-managed authorized_keys lines.
// The CLI writes this before GrantNodeSSHAccess so devplane need not modify the file.
func DevplaneAuthorizedKeysComment(portID, userID string) string {
	return fmt.Sprintf("#brev-portID:%s,brev-userID:%s", portID, userID)
}

// BrevAuthorizedKey represents a single Brev-managed key found in authorized_keys.
type BrevAuthorizedKey struct {
	Line       string // full line from authorized_keys
	KeyContent string // key type + material (and optional ssh comment), without brev suffix
	PortID     string // from devplane #brev-portID:...
	UserID     string // from devplane brev-userID:... or legacy user_id=
}

func isBrevManagedAuthorizedKeysLine(line string) bool {
	return strings.Contains(line, BrevKeyPrefixLegacy) || strings.Contains(line, "#brev-portID:")
}

func parseBrevAuthorizedKeyLine(trimmed string) BrevAuthorizedKey {
	bk := BrevAuthorizedKey{Line: trimmed}

	if idx := strings.Index(trimmed, "#brev-portID:"); idx >= 0 {
		bk.KeyContent = strings.TrimSpace(trimmed[:idx])
		tag := trimmed[idx+1:]
		for _, part := range strings.Split(tag, ",") {
			part = strings.TrimSpace(part)
			switch {
			case strings.HasPrefix(part, brevPortIDField):
				bk.PortID = strings.TrimPrefix(part, brevPortIDField)
			case strings.HasPrefix(part, brevUserIDField):
				bk.UserID = strings.TrimPrefix(part, brevUserIDField)
			}
		}
		return bk
	}

	if idx := strings.Index(trimmed, " "+BrevKeyPrefixLegacy); idx >= 0 {
		bk.KeyContent = strings.TrimSpace(trimmed[:idx])
		tag := trimmed[idx+1:]
		if uidIdx := strings.Index(tag, "user_id="); uidIdx >= 0 {
			rest := tag[uidIdx+len("user_id="):]
			if spIdx := strings.Index(rest, " "); spIdx >= 0 {
				bk.UserID = rest[:spIdx]
			} else {
				bk.UserID = rest
			}
		}
		return bk
	}

	bk.KeyContent = trimmed
	return bk
}

// ListBrevAuthorizedKeys reads ~/.ssh/authorized_keys and returns Brev-managed lines.
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
		if !isBrevManagedAuthorizedKeysLine(line) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		keys = append(keys, parseBrevAuthorizedKeyLine(trimmed))
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

func openPortForSSHAccess(
	ctx context.Context,
	t *terminal.Terminal,
	nodeClients externalnode.NodeClientFactory,
	tokenProvider externalnode.TokenProvider,
	reg *DeviceRegistration,
) (string, error) {
	t.Vprint("")
	port, err := PromptSSHPort(t)
	if err != nil {
		return "", fmt.Errorf("invalid port: %w", err)
	}
	return OpenSSHPort(ctx, t, nodeClients, tokenProvider, reg, port)
}

// OpenSSHPort calls the OpenPort RPC to allocate a port on the node for SSH access.
// The call is idempotent — if the port is already open, the server returns the existing allocation.
func OpenSSHPort(
	ctx context.Context,
	t *terminal.Terminal,
	nodeClients externalnode.NodeClientFactory,
	tokenProvider externalnode.TokenProvider,
	reg *DeviceRegistration,
	port int32,
) (string, error) {
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid port %d: port must be between 1 and 65535", port)
	}
	client := nodeClients.NewNodeClient(tokenProvider, config.GlobalConfig.GetBrevPublicAPIURL())
	brevPort, err := client.OpenPort(ctx, connect.NewRequest(&nodev1.OpenPortRequest{
		ExternalNodeId: reg.ExternalNodeID,
		Protocol:       nodev1.PortProtocol_PORT_PROTOCOL_TCP,
		PortNumber:     port,
	}))
	if err != nil {
		return "", fmt.Errorf("failed to allocate port: %w", err)
	}
	t.Vprintf("  Port %d allocated (%s).\n", port, FormatPortLabel(brevPort.Msg.GetPort()))
	return brevPort.Msg.GetPort().GetPortId(), nil
}

// SetupAndRegisterNodeSSHAccess installs the user's public key in authorized_keys
// with the devplane comment tag, then calls GrantNodeSSHAccess. The key must be
// present locally before devplane can grant; matching tags make the grant a no-op
// on the authorized_keys file.
func SetupAndRegisterNodeSSHAccess(
	ctx context.Context,
	t *terminal.Terminal,
	nodeClients externalnode.NodeClientFactory,
	tokenProvider externalnode.TokenProvider,
	reg *DeviceRegistration,
	targetUser *entity.User,
	linuxUsername string,
	brevPortID string,
) error {
	if brevPortID == "" {
		return fmt.Errorf("port id is required to grant SSH access")
	}

	osUser, lookupErr := user.Lookup(linuxUsername)
	if lookupErr != nil {
		return fmt.Errorf("could not look up local user %q: %w", linuxUsername, lookupErr)
	}

	var keyAdded bool
	if targetUser.PublicKey != "" {
		added, installErr := InstallAuthorizedKey(osUser, targetUser.PublicKey, brevPortID, targetUser.ID)
		if installErr != nil {
			return fmt.Errorf("failed to install SSH public key: %w", installErr)
		}
		keyAdded = added
		if added {
			t.Vprint("  Public key added to authorized_keys.")
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
			PortId:         brevPortID,
			UserId:         targetUser.ID,
			LinuxUser:      linuxUsername,
		}))
		if err != nil {
			// Retryable error
			var connectErr *connect.Error
			if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeInternal {
				return fmt.Errorf("failed to grant SSH access (transient): %w", err)
			}

			// Permanent error — roll back only the line we added for this port
			if keyAdded && targetUser.PublicKey != "" {
				line := targetUser.PublicKey + " " + DevplaneAuthorizedKeysComment(brevPortID, targetUser.ID)
				if rerr := RemoveAuthorizedKeyLine(osUser, line); rerr != nil {
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
// they press Enter or leave it empty. Re-prompts on invalid input until a valid
// port is provided. Only returns an error for unrecoverable I/O failures.
func PromptSSHPort(t *terminal.Terminal) (int32, error) {
	if testSSHPort != nil {
		return *testSSHPort, nil
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		t.Vprintf("  %s ", t.Green("SSH port (default 22):"))
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
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Invalid port %q: port must be a number between 1 and 65535", portStr)))
			continue
		}
		if n < 1 || n > 65535 { // explicit gate even though ParseUint will already fail values out of range
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Invalid port %q: port must be between 1 and 65535", portStr)))
			continue
		}
		return int32(n), nil
	}
}

// InstallAuthorizedKey adds an authorized_keys line for this port and user. The same
// public key may appear on multiple lines (one per port). Untagged or legacy-tagged
// lines are upgraded in place; lines that already have a #brev-portID tag are left
// alone and a new line is appended.
func InstallAuthorizedKey(u *user.User, pubKey, portID, brevUserID string) (bool, error) {
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

	taggedLine := pubKey + " " + DevplaneAuthorizedKeysComment(portID, brevUserID)

	if strings.Contains(string(existing), taggedLine) {
		return false, nil
	}

	lines := strings.Split(string(existing), "\n")
	var out []string
	upgraded := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !upgraded && strings.Contains(trimmed, pubKey) && !strings.Contains(trimmed, "#brev-portID:") {
			out = append(out, taggedLine)
			upgraded = true
			continue
		}
		out = append(out, trimmed)
	}

	content := strings.Join(out, "\n")
	if len(out) > 0 {
		content += "\n"
	}
	if !upgraded {
		content += taggedLine + "\n"
	}

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

// RemoveBrevAuthorizedKeys removes all Brev-managed SSH keys from authorized_keys.
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
		if isBrevManagedAuthorizedKeysLine(line) {
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
