// Package register provides the brev register command for device registration
package register

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// RegisterStore defines the store methods needed by the register command.
type RegisterStore interface {
	GetCurrentUser() (*entity.User, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetBrevHomePath() (string, error)
	GetAccessToken() (string, error)
}

// OSFileReader reads files from the real OS filesystem.
type OSFileReader struct{}

func (r OSFileReader) ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return data, nil
}

// NetBirdManager installs and uninstalls the NetBird network agent.
type NetBirdManager interface {
	Install() error
	Uninstall() error
}

// ManagedSSHDaemon installs and uninstalls a brev-managed sshd instance.
type ManagedSSHDaemon interface {
	Install() error
	Uninstall() error
}

// SetupRunner runs a setup script on the local machine.
type SetupRunner interface {
	RunSetup(script string) error
}

// registerDeps bundles the side-effecting dependencies of runRegister so they
// can be replaced in tests.
type registerDeps struct {
	platform          externalnode.PlatformChecker
	prompter          terminal.Confirmer
	netbird       NetBirdManager
	setupRunner   SetupRunner
	nodeClients       externalnode.NodeClientFactory
	commandRunner     CommandRunner
	fileReader        FileReader
	registrationStore RegistrationStore
}

func defaultRegisterDeps(brevHome string) registerDeps {
	return registerDeps{
		platform:          LinuxPlatform{},
		prompter:          TerminalPrompter{},
		netbird:       Netbird{},
		setupRunner:   ShellSetupRunner{},
		nodeClients:       DefaultNodeClientFactory{},
		commandRunner:     ExecCommandRunner{},
		fileReader:        OSFileReader{},
		registrationStore: NewFileRegistrationStore(brevHome),
	}
}

var (
	registerLong = `Register your device with NVIDIA Brev

This command sets up network connectivity and registers this machine with Brev.`

	registerExample = `  brev register "My DGX Spark"`
)

func NewCmdRegister(t *terminal.Terminal, store RegisterStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "register <name>",
		DisableFlagsInUseLine: true,
		Short:                 "Register this device with Brev",
		Long:                  registerLong,
		Example:               registerExample,
		Args:                  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			brevHome, err := store.GetBrevHomePath()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return runRegister(cmd.Context(), t, store, args[0], defaultRegisterDeps(brevHome))
		},
	}

	return cmd
}

func runRegister(ctx context.Context, t *terminal.Terminal, s RegisterStore, name string, deps registerDeps) error { //nolint:funlen // registration flow
	if !deps.platform.IsCompatible() {
		return breverrors.New("brev register is only supported on Linux")
	}

	alreadyRegistered, err := deps.registrationStore.Exists()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if alreadyRegistered {
		reg, loadErr := deps.registrationStore.Load()
		if loadErr != nil {
			return fmt.Errorf("this machine is already registered but the registration file could not be read: %w", loadErr)
		}
		return checkExistingRegistration(ctx, t, s, deps, reg)
	}

	brevUser, err := s.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	org, err := getOrgToRegisterFor(s)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	osUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green("Registering your device with Brev"))
	t.Vprint("")
	t.Vprintf("  Name:         %s\n", t.Yellow(name))
	t.Vprintf("  Organization: %s\n", org.Name)
	t.Vprintf("  Registering for Linux user:   %s\n", osUser.Username)
	t.Vprint("")
	t.Vprint("This will perform the following steps:")
	t.Vprint("  1. Set up Brev tunnel")
	t.Vprint("  2. Collect hardware profile")
	t.Vprint("  3. Register this machine with Brev")
	t.Vprint("")

	if !deps.prompter.ConfirmYesNo("Proceed with registration?") {
		t.Vprint("Registration canceled.")
		return nil
	}

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 1/3] Setting up Brev tunnel..."))
	if err := deps.netbird.Install(); err != nil {
		return fmt.Errorf("brev tunnel setup failed: %w", err)
	}
	t.Vprint(t.Green("  Brev tunnel ready."))

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 2/3] Collecting hardware profile..."))
	t.Vprint("")

	nodeSpec, err := CollectHardwareProfile(deps.commandRunner, deps.fileReader)
	if err != nil {
		return fmt.Errorf("failed to collect hardware profile: %w", err)
	}

	t.Vprint("  Hardware profile:")
	t.Vprint(FormatNodeSpec(nodeSpec))

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 3/3] Registering with Brev..."))

	deviceID := uuid.New().String()
	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	addResp, err := client.AddNode(ctx, connect.NewRequest(&nodev1.AddNodeRequest{
		OrganizationId: org.ID,
		Name:           name,
		DeviceId:       deviceID,
		NodeSpec:       toProtoNodeSpec(nodeSpec),
	}))
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	node := addResp.Msg.GetExternalNode()
	reg := &DeviceRegistration{
		ExternalNodeID: node.GetExternalNodeId(),
		DisplayName:    name,
		OrgID:          org.ID,
		DeviceID:       deviceID,
		RegisteredAt:   time.Now().UTC().Format(time.RFC3339),
		NodeSpec:       *nodeSpec,
	}
	if err := deps.registrationStore.Save(reg); err != nil {
		return fmt.Errorf("node registered but failed to save locally: %w", err)
	}

	t.Vprint(t.Green("  Registration complete."))

	runSetup(node, t, deps)

	if deps.prompter.ConfirmYesNo("Would you like to enable SSH access to this device?") {
		grantSSHAccess(ctx, t, deps, s, reg, brevUser, osUser)
	}

	return nil
}

func getOrgToRegisterFor(s RegisterStore) (*entity.Organization, error) {
	org, err := s.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, fmt.Errorf("no organization found; please create or join an organization first")
	}

	return org, nil
}

// checkExistingRegistration verifies connectivity for an already-registered node.
// It calls GetNode to check the server-side NetworkMemberStatus and ensures the
// local netbird service is running, starting it if necessary. Returns nil if
// the node is healthy, or an error describing what's wrong.
func checkExistingRegistration(ctx context.Context, t *terminal.Terminal, s RegisterStore, deps registerDeps, reg *DeviceRegistration) error {
	t.Vprint("")
	t.Vprintf("  This machine is already registered as %s (%s).\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprint("  Checking connectivity...")
	t.Vprint("")

	// Check server-side connectivity status via GetNode.
	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	resp, err := client.GetNode(ctx, connect.NewRequest(&nodev1.GetNodeRequest{
		ExternalNodeId: reg.ExternalNodeID,
		OrganizationId: reg.OrgID,
	}))
	if err != nil {
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: could not fetch node status: %v", err)))
	} else if node := resp.Msg.GetExternalNode(); node == nil {
		t.Vprintf("  %s\n", t.Yellow("Warning: could not fetch node connectivity info"))
	} else {
		ci := node.GetConnectivityInfo()
		if ci != nil && ci.GetStatus() == nodev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_CONNECTED {
			t.Vprint(t.Green("  Node is connected."))
			t.Vprint("")
			t.Vprint("  Run 'brev deregister' first if you want to re-register.")
			return nil
		}
		t.Vprintf("  Node status: %s\n", externalnode.FriendlyNetworkStatus(ci.GetStatus()))
	}

	// Check local netbird service and start it if down.
	t.Vprint("  Checking local Brev tunnel...")
	if ensureNetbirdRunning(t) {
		t.Vprint(t.Green("  Brev tunnel is running."))
	}

	t.Vprint("")
	t.Vprint("  Run 'brev deregister' first if you want to re-register.")
	return nil
}

// ensureNetbirdRunning checks if the netbird systemd service is active and
// attempts to start it if it is not. It also checks the netbird peer
// connection status and runs "netbird up" if the peer is disconnected.
// Returns true if the service is running and connected after the check.
func ensureNetbirdRunning(t *terminal.Terminal) bool {
	out, err := exec.Command("systemctl", "is-active", "netbird").Output() //nolint:gosec // fixed service name
	if err != nil || strings.TrimSpace(string(out)) != "active" {
		t.Vprintf("  %s\n", t.Yellow("Brev tunnel service is not running. Attempting to start..."))
		if startErr := exec.Command("sudo", "systemctl", "start", "netbird").Run(); startErr != nil { //nolint:gosec // fixed service name
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to start Brev tunnel service: %v", startErr)))
			return false
		}
		t.Vprint(t.Green("  Brev tunnel service started."))
	}

	// Service is running — now check peer connection status.
	statusOut, err := exec.Command("netbird", "status").Output() //nolint:gosec // fixed command
	if err != nil {
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: could not check Brev tunnel status: %v", err)))
		return true // service is running, just can't confirm peer status
	}

	if netbirdManagementConnected(string(statusOut)) {
		return true
	}

	t.Vprintf("  %s\n", t.Yellow("Brev tunnel peer is disconnected. Reconnecting..."))
	if upErr := exec.Command("sudo", "netbird", "up").Run(); upErr != nil { //nolint:gosec // fixed command
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to reconnect Brev tunnel: %v", upErr)))
		return false
	}
	t.Vprint(t.Green("  Brev tunnel reconnected."))
	return true
}

// netbirdManagementConnected parses "netbird status" output and returns true
// when the Management line reports "Connected".
func netbirdManagementConnected(statusOutput string) bool {
	for _, line := range strings.Split(statusOutput, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Management:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Management:")) == "Connected"
		}
	}
	return false
}

func runSetup(node *nodev1.ExternalNode, t *terminal.Terminal, deps registerDeps) {
	ci := node.GetConnectivityInfo()
	if ci == nil || ci.GetRegistrationCommand() == "" {
		t.Vprintf("  %s\n", t.Yellow("Warning: Brev tunnel setup failed, please try again."))
	} else {
		if err := deps.setupRunner.RunSetup(ci.GetRegistrationCommand()); err != nil {
			t.Vprintf("  Warning: setup command failed: %v\n", err)
		} else {
			// netbird up reconfigures network routes; give them a moment
			// to settle before making further RPC calls.
			time.Sleep(2 * time.Second)
		}
	}
}

func grantSSHAccess(ctx context.Context, t *terminal.Terminal, deps registerDeps, tokenProvider externalnode.TokenProvider, reg *DeviceRegistration, brevUser *entity.User, osUser *user.User) {
	t.Vprint("")
	t.Vprint(t.Green("Enabling SSH access on this device"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Brev user:  %s\n", brevUser.ID)
	t.Vprintf("  Linux user: %s\n", osUser.Username)
	t.Vprint("")

	err := GrantSSHAccessToNode(ctx, t, deps.nodeClients, tokenProvider, reg, brevUser, osUser)
	if err != nil {
		t.Vprint("  Retrying in 3 seconds...")
		time.Sleep(3 * time.Second)
		err = GrantSSHAccessToNode(ctx, t, deps.nodeClients, tokenProvider, reg, brevUser, osUser)
	}
	if err != nil {
		t.Vprintf("  Warning: %v\n", err)
		return
	}

	t.Vprint(t.Green(fmt.Sprintf("SSH access enabled. You can now SSH to this device via: brev shell %s", reg.DisplayName)))
}
