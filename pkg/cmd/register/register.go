// Package register provides the brev register command for device registration
package register

import (
	"context"
	"fmt"
	"os/user"
	"strings"
	"time"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/briandowns/spinner"
	"github.com/google/uuid"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/names"
	"github.com/brevdev/brev-cli/pkg/sudo"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// RegisterStore defines the store methods needed by the register command.
type RegisterStore interface {
	GetCurrentUser() (*entity.User, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetOrganizationsByName(name string) ([]entity.Organization, error)
	ListOrganizations() ([]entity.Organization, error)
	GetAccessToken() (string, error)
}

// NetBirdManager installs, uninstalls, and monitors the NetBird network agent.
type NetBirdManager interface {
	Install() error
	Uninstall() error
	// EnsureRunning checks whether the NetBird service is active and
	// connected, starting or reconnecting it if needed. Returns nil when
	// the tunnel is healthy.
	EnsureRunning() error
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
	selector          terminal.Selector
	netbird           NetBirdManager
	setupRunner       SetupRunner
	nodeClients       externalnode.NodeClientFactory
	hardwareProfiler  HardwareProfiler
	registrationStore RegistrationStore
}

func defaultRegisterDeps() registerDeps {
	p := TerminalPrompter{}
	return registerDeps{
		platform:          LinuxPlatform{},
		prompter:          p,
		selector:          p,
		netbird:           Netbird{},
		setupRunner:       ShellSetupRunner{},
		nodeClients:       DefaultNodeClientFactory{},
		hardwareProfiler:  &SystemHardwareProfiler{},
		registrationStore: NewFileRegistrationStore(),
	}
}

var (
	registerLong = `Register your device with NVIDIA Brev

This command sets up network connectivity and registers this machine with Brev.

Two modes are supported:
  • Interactive (default): run 'brev register' (or 'brev register <name>') and follow prompts for org and options.
  • Non-interactive: use any of --name, --org, --enable-ssh, or --ssh-port (or --non-interactive). No prompts; --name and --org are required. Use for scripts/CI.`

	registerExample = `  # Interactive (prompts for org, confirmations)
  brev register
  brev register "My DGX Spark"

  # Non-interactive (any flag implies no prompts; --name and --org required)
  brev register --name my-node --org my-org
  brev register --name my-node --org my-org --enable-ssh --ssh-port 22`
)

func NewCmdRegister(t *terminal.Terminal, store RegisterStore) *cobra.Command {
	var orgFlag string
	var nonInteractive bool
	var nameFlag string
	var enableSSH bool
	var sshPort int

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "register [name]",
		DisableFlagsInUseLine: true,
		Short:                 "Register this device with Brev",
		Long:                  registerLong,
		Example:               registerExample,
		Args:                  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := nameFlag
			if name == "" && len(args) > 0 {
				name = args[0]
			}
			// Non-interactive if explicit flag or any register-specific flag is set (implies script/CI).
			flagDriven := nonInteractive ||
				nameFlag != "" ||
				orgFlag != "" ||
				enableSSH ||
				cmd.Flags().Changed("ssh-port")
			if flagDriven {
				return runRegisterFlagDriven(cmd.Context(), t, store, name, orgFlag, enableSSH, int32(sshPort), defaultRegisterDeps())
			}
			return runRegisterPromptDriven(cmd.Context(), t, store, name, orgFlag, defaultRegisterDeps())
		},
	}

	cmd.Flags().StringVarP(&orgFlag, "org", "o", "", "organization name (required when using non-interactive mode)")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "non-interactive mode (also implied by --name, --org, --enable-ssh, or --ssh-port)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "device name (required when using non-interactive mode)")
	cmd.Flags().BoolVar(&enableSSH, "enable-ssh", false, "enable SSH access after registration (non-interactive mode)")
	cmd.Flags().IntVar(&sshPort, "ssh-port", 22, "SSH port when using --enable-ssh")

	return cmd
}

func runRegister(ctx context.Context, t *terminal.Terminal, s RegisterStore, name string, orgName string, deps registerDeps) error {
	return runRegisterPromptDriven(ctx, t, s, name, orgName, deps)
}

// runRegisterSteps performs netbird install, hardware profile, AddNode, save registration, and runSetup.
// It does not prompt or enable SSH. Used by both flag-driven and prompt-driven flows.
func runRegisterSteps(ctx context.Context, t *terminal.Terminal, s RegisterStore, name string, org *entity.Organization, deps registerDeps) (*DeviceRegistration, error) {
	const clearLine = "\033[2K\n"
	t.Vprint("")

	// Stop spinner (and clear its line) before any of our prints so stdout and stderr don't interleave.
	var sp *spinner.Spinner
	stopSpinner := func() {
		if sp != nil {
			sp.FinalMSG = clearLine
			sp.Stop()
			sp = nil
		}
	}
	defer stopSpinner()

	t.Vprint(t.Yellow("[Step 1/3] Setting up Brev tunnel..."))
	sp = t.NewSpinner()
	sp.Suffix = " Registering device..."
	sp.Start()
	err := deps.netbird.Install()
	stopSpinner()
	if err != nil {
		return nil, fmt.Errorf("brev tunnel setup failed: %w", err)
	}
	t.Vprintf("%s  Brev tunnel ready.\n", t.Green("  ✓"))

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 2/3] Collecting hardware profile..."))
	sp = t.NewSpinner()
	sp.Suffix = " Registering device..."
	sp.Start()
	hwProfile, err := deps.hardwareProfiler.Profile()
	stopSpinner()
	if err != nil {
		return nil, fmt.Errorf("failed to collect hardware profile: %w", err)
	}
	t.Vprintf("%s  Hardware profile collected.\n", t.Green("  ✓"))
	t.Vprint("")
	t.Vprint("  Hardware profile:")
	t.Vprint(FormatHardwareProfile(hwProfile))

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 3/3] Registering with Brev..."))
	sp = t.NewSpinner()
	sp.Suffix = " Registering device..."
	sp.Start()
	deviceID := uuid.New().String()
	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	addResp, err := client.AddNode(ctx, connect.NewRequest(&nodev1.AddNodeRequest{
		OrganizationId: org.ID,
		Name:           name,
		DeviceId:       deviceID,
		NodeSpec:       toProtoNodeSpec(hwProfile),
	}))
	if err != nil {
		stopSpinner()
		return nil, fmt.Errorf("failed to register node: %w", err)
	}

	node := addResp.Msg.GetExternalNode()
	reg := &DeviceRegistration{
		ExternalNodeID:  node.GetExternalNodeId(),
		DisplayName:     name,
		OrgID:           org.ID,
		OrgName:         org.Name,
		DeviceID:        deviceID,
		RegisteredAt:    time.Now().UTC().Format(time.RFC3339),
		HardwareProfile: *hwProfile,
	}
	if err := deps.registrationStore.Save(reg); err != nil {
		stopSpinner()
		return nil, fmt.Errorf("node registered but failed to save locally: %w", err)
	}

	runSetup(node, t, deps)
	stopSpinner()
	t.Vprintf("%s  Node registered.\n", t.Green("  ✓"))
	t.Vprintf("%s  Registration complete.\n", t.Green("  ✓"))
	return reg, nil
}

// resolveOrgPromptDriven resolves organization for prompt-driven flow: by name if --org given, else always list and select with arrow keys.
func resolveOrgPromptDriven(t *terminal.Terminal, s RegisterStore, orgName string, deps registerDeps) (*entity.Organization, error) {
	if orgName != "" {
		orgs, err := s.GetOrganizationsByName(orgName)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if len(orgs) == 0 {
			return nil, fmt.Errorf("no organization found with name %q", orgName)
		}
		if len(orgs) > 1 {
			return nil, fmt.Errorf("multiple organizations found with name %q", orgName)
		}
		return &orgs[0], nil
	}

	list, err := s.ListOrganizations()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no organization found; please create or join an organization first")
	}

	t.Vprint("")
	names := make([]string, len(list))
	for i := range list {
		names[i] = list[i].Name
	}
	chosen := deps.selector.Select("Select organization", names)
	for i := range list {
		if list[i].Name == chosen {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("selected organization not found")
}

func runRegisterPromptDriven(ctx context.Context, t *terminal.Terminal, s RegisterStore, name string, orgName string, deps registerDeps) error {
	if !deps.platform.IsCompatible() {
		return breverrors.New("brev register is only supported on Linux")
	}

	if err := sudo.Gate(t, deps.prompter, "Device registration"); err != nil {
		return fmt.Errorf("sudo issue: %w", err)
	}

	// Ensure user is logged in before any other prompts (email/login happens here if needed).
	if _, err := s.GetCurrentUser(); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	alreadyRegistered, err := deps.registrationStore.Exists()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if alreadyRegistered {
		return checkExistingRegistration(ctx, t, s, name, deps)
	}

	if name == "" {
		t.Vprint("")
		name = terminal.PromptGetInput(terminal.PromptContent{
			Label:      "Device name",
			ErrorMsg:   "name is required",
			AllowEmpty: false,
		})
		name = strings.TrimSpace(name)
	}
	if err := names.ValidateNodeName(name); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint("")
	org, err := resolveOrgPromptDriven(t, s, orgName, deps)
	if err != nil {
		return err
	}

	brevUser, err := s.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	osUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint(t.White("  Registering your device with Brev"))
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint("")
	t.Vprint(t.Green("  Please confirm before continuing:"))
	t.Vprint("")
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Device name:")), t.BoldBlue(name))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Organization:")), t.BoldBlue(org.Name))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Linux user:")), t.BoldBlue(osUser.Username))
	t.Vprint("")
	t.Vprint(t.Yellow("  This will:"))
	t.Vprint("    1. Set up Brev tunnel")
	t.Vprint("    2. Collect hardware profile")
	t.Vprint("    3. Register this machine with Brev")
	t.Vprint("")

	if !deps.prompter.ConfirmYesNo("Proceed with registration?") {
		t.Vprint("Registration canceled.")
		return nil
	}

	reg, err := runRegisterSteps(ctx, t, s, name, org, deps)
	if err != nil {
		return err
	}

	if deps.prompter.ConfirmYesNo("Would you like to enable SSH access to this device?") {
		if err := grantSSHAccessWithPort(ctx, t, deps, s, reg, brevUser, osUser, 0); err != nil {
			t.Vprintf("  Warning: SSH access not granted: %v\n", err)
		}
	}

	return nil
}

func runRegisterFlagDriven(ctx context.Context, t *terminal.Terminal, s RegisterStore, name string, orgName string, enableSSH bool, sshPort int32, deps registerDeps) error {
	if !deps.platform.IsCompatible() {
		return breverrors.New("brev register is only supported on Linux")
	}

	if name == "" || orgName == "" {
		return fmt.Errorf("in non-interactive mode --name and --org are required")
	}

	alreadyRegistered, err := deps.registrationStore.Exists()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if alreadyRegistered {
		return checkExistingRegistration(ctx, t, s, name, deps)
	}

	if err := names.ValidateNodeName(name); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	org, err := getOrgToRegisterFor(s, orgName)
	if err != nil {
		return err
	}

	reg, err := runRegisterSteps(ctx, t, s, name, org, deps)
	if err != nil {
		return err
	}

	if enableSSH {
		brevUser, err := s.GetCurrentUser()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		osUser, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed to determine current Linux user: %w", err)
		}
		if err := grantSSHAccessWithPort(ctx, t, deps, s, reg, brevUser, osUser, sshPort); err != nil {
			return err
		}
	}

	return nil
}

func getOrgToRegisterFor(s RegisterStore, orgName string) (*entity.Organization, error) {
	if orgName != "" {
		orgs, err := s.GetOrganizationsByName(orgName)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if len(orgs) == 0 {
			return nil, fmt.Errorf("no organization found with name %q", orgName)
		}
		if len(orgs) > 1 {
			return nil, fmt.Errorf("multiple organizations found with name %q", orgName)
		}
		return &orgs[0], nil
	}

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
func checkExistingRegistration(ctx context.Context, t *terminal.Terminal, s RegisterStore, name string, deps registerDeps) error {
	reg, loadErr := deps.registrationStore.Load()
	if loadErr != nil {
		return fmt.Errorf("this machine is already registered but the registration file could not be read: %w", loadErr)
	}
	if name != "" && name != reg.DisplayName {
		// TODO maybe allow for a name change
		t.Vprintf("This machine is already registered as %q.\n", reg.DisplayName)
		t.Vprint("Run 'brev deregister' first if you want to re-register with a different name.")
		t.Vprint("")
		t.Vprintf("If you are having tunnel issues, run 'brev register %q' to reconnect.\n", reg.DisplayName)
		return nil
	}

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
	if err := deps.netbird.EnsureRunning(); err != nil {
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: %v", err)))
	} else {
		t.Vprint(t.Green("  Brev tunnel is running."))
	}

	t.Vprint("")
	t.Vprint("  Run 'brev deregister' first if you want to re-register.")
	return nil
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

// grantSSHAccessWithPort enables SSH; if port is 0, prompts for port (prompt-driven). Otherwise uses the given port (flag-driven).
func grantSSHAccessWithPort(ctx context.Context, t *terminal.Terminal, deps registerDeps, tokenProvider externalnode.TokenProvider, reg *DeviceRegistration, brevUser *entity.User, osUser *user.User, port int32) error {
	t.Vprint("")
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint(t.White("  Enabling SSH access on this device"))
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint("")
	t.Vprint(t.Green("  Please confirm before continuing:"))
	t.Vprint("")
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Device name:")), t.BoldBlue(reg.DisplayName))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Node ID:")), t.BoldBlue(reg.ExternalNodeID))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Brev user:")), t.BoldBlue(brevUser.ID))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Linux user:")), t.BoldBlue(osUser.Username))

	var err error
	if port == 0 {
		t.Vprint("")
		port, err = PromptSSHPort(t)
		if err != nil {
			return fmt.Errorf("SSH port: %w", err)
		}
	} else {
		t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "SSH port:")), t.BoldBlue(fmt.Sprintf("%d", port)))
	}
	t.Vprint("")

	if err := OpenSSHPort(ctx, t, deps.nodeClients, tokenProvider, reg, port); err != nil {
		return fmt.Errorf("allocate SSH port failed: %w", err)
	}

	err = GrantSSHAccessToNode(ctx, t, deps.nodeClients, tokenProvider, reg, brevUser, osUser)
	if err != nil {
		return fmt.Errorf("grant SSH failed: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green(fmt.Sprintf("SSH access enabled. You can now SSH to this device via: brev shell %s", reg.DisplayName)))
	return nil
}
