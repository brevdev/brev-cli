// Package register provides the brev register command for device registration
package register

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/externalnode/helpers"
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
	gater             sudo.Gater
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
		gater:             sudo.Default,
		netbird:           Netbird{},
		setupRunner:       ShellSetupRunner{},
		nodeClients:       DefaultNodeClientFactory{},
		hardwareProfiler:  &SystemHardwareProfiler{},
		registrationStore: NewFileRegistrationStore(),
	}
}

type OrgLister interface {
	ListOrganizations() ([]entity.Organization, error)
}

var (
	registerLong = `Register your device with NVIDIA Brev

This command registers this machine with Brev and brings up the Brev tunnel.

Two modes are supported:
  • Interactive (default): run 'brev register' with no flags and follow prompts for device name and org.
  • Non-interactive: use --name and --org. No prompts; both are required.
    Use for scripts/CI.
`

	registerExample = `  # Interactive (prompts for device name, org, confirmations)
  brev register

  # Non-interactive (--name and --org required)
  brev register --name my-node --org my-org`
)

func NewCmdRegister(t *terminal.Terminal, store RegisterStore) *cobra.Command {
	var orgFlag string
	var nameFlag string
	var sshPort int // deprecated
	var approveFlag bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "register",
		DisableFlagsInUseLine: true,
		Short:                 "Register this device with Brev",
		Long:                  registerLong,
		Example:               registerExample,
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			interactive := nameFlag == "" && orgFlag == "" && sshPort == 0
			opts := registerOpts{
				interactive: interactive,
				name:        nameFlag,
				orgName:     orgFlag,
				skipConfirm: approveFlag,
			}
			return runRegister(cmd.Context(), t, store, opts, defaultRegisterDeps())
		},
	}

	cmd.Flags().StringVarP(&orgFlag, "org", "o", "", "organization name (required when using non-interactive mode)")
	cmd.Flags().StringVarP(&nameFlag, "name", "n", "", "device name (required when using non-interactive mode)")
	cmd.Flags().IntVarP(&sshPort, "ssh-port", "p", 0, "SSH port (if ssh access is desired)")
	cmd.Flags().BoolVar(&approveFlag, "approve", false, "skip all confirmation prompts (assume yes)")
	_ = cmd.Flags().MarkDeprecated("ssh-port", "use 'brev enable-ssh' after registration to enable SSH access")

	return cmd
}

// registerOpts carries mode and inputs: when interactive, name/orgName are from prompts; otherwise from flags.
type registerOpts struct {
	interactive bool
	name        string
	orgName     string
	skipConfirm bool
}

func runRegister(ctx context.Context, t *terminal.Terminal, s RegisterStore, opts registerOpts, deps registerDeps) error { //nolint:gocognit,gocyclo,funlen // ok
	if !deps.platform.IsCompatible() {
		return breverrors.New("brev register is only supported on Linux")
	}
	// Always gate on sudo; skip confirmation prompt when non-interactive or --approve.
	if err := deps.gater.Gate(t, deps.prompter, "Device registration", !opts.interactive || opts.skipConfirm); err != nil {
		return fmt.Errorf("sudo issue: %w", err)
	}

	if !opts.interactive {
		if opts.name == "" || opts.orgName == "" {
			return fmt.Errorf("in non-interactive mode --name and --org are required")
		}
	}
	// Verify the user is authenticated before performing any local side effects.
	if _, err := s.GetCurrentUser(); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var intendedOrg *entity.Organization
	if !opts.interactive {
		o, err := resolveOrg(s, opts.orgName)
		if err != nil {
			return err
		}
		intendedOrg = o
	}

	// Check for an existing registration (confirmed or in-progress).
	exists, err := deps.registrationStore.Exists()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if exists {
		reg, err := deps.registrationStore.Load(true)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if intendedOrg != nil && intendedOrg.ID != reg.OrgID {
			return orgMismatchError(reg, intendedOrg)
		}
		if reg.Status == RegistrationStatusPending {
			return resumeRegistration(ctx, t, s, deps, reg)
		}
		return checkExistingRegistration(ctx, t, s, deps, reg)
	}

	var name string
	if opts.interactive {
		t.Vprint("")
		name = terminal.PromptGetInput(terminal.PromptContent{
			Label:      "Device name",
			ErrorMsg:   "name is required",
			AllowEmpty: false,
		})
		name = strings.TrimSpace(name)
	} else {
		name = opts.name
	}
	if err := names.ValidateNodeName(name); err != nil {
		return err //nolint:wrapcheck // do not present stack trace for this error
	}

	// Non-interactive already resolved intendedOrg above; interactive prompts.
	var org *entity.Organization
	if intendedOrg != nil {
		org = intendedOrg
	} else {
		t.Vprint("")
		org, err = resolveOrgInteractive(t, s, deps)
	}
	if err != nil {
		return err
	}

	t.Vprint("")
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint(t.White("  Registering your device with Brev"))
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint("")
	if opts.interactive && !opts.skipConfirm {
		t.Vprint(t.Green("  Please confirm before continuing:"))
		t.Vprint("")
	}
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Device:")), t.BoldBlue(name))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Organization:")), t.BoldBlue(org.Name+" ("+org.ID+")"))
	t.Vprint("")
	t.Vprint(t.Yellow("  This will:"))
	t.Vprint("    1. Download and install Brev tunnel")
	t.Vprint("    2. Collect hardware profile")
	t.Vprint("    3. Register this machine with Brev")
	t.Vprint("    4. Store registration data")
	t.Vprint("    5. Connect device to Brev")
	t.Vprint("")

	if opts.interactive {
		if !opts.skipConfirm && !deps.prompter.ConfirmYesNo("Proceed with registration?") {
			t.Vprint("Registration canceled.")
			return nil
		}
	}

	// Generate the device ID here so a retry reuses it (AddNode is idempotent on device_id).
	deviceID := uuid.New().String()
	return runRegisterSteps(ctx, t, s, name, org, deps, deviceID)
}

// runRegisterSteps runs tunnel install, hardware profile, AddNode, persist, and setup
func runRegisterSteps(ctx context.Context, t *terminal.Terminal, s RegisterStore, name string, org *entity.Organization, deps registerDeps, deviceID string) error {
	t.Vprint("")

	t.Vprint(t.Yellow("[Step 1/5] Downloading and installing Brev tunnel..."))
	err := deps.netbird.Install()
	if err != nil {
		return fmt.Errorf("brev tunnel setup failed: %w", err)
	}
	t.Vprintf("%s  Brev tunnel ready.\n", t.Green("  ✓"))

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 2/5] Collecting hardware profile..."))
	hwProfile, err := deps.hardwareProfiler.Profile()
	if err != nil {
		return fmt.Errorf("failed to collect hardware profile: %w", err)
	}
	t.Vprintf("%s  Hardware profile collected.\n", t.Green("  ✓"))
	t.Vprint("")
	t.Vprint("  Hardware profile:")
	t.Vprint(FormatHardwareProfile(hwProfile))

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 3/5] Registering device with Brev..."))

	// A pending record written before AddNode (see resumeRegistration) makes the
	// flow resumable: a crash/timeout after the node exists is retried with the
	// same device ID, and AddNode is idempotent on device_id.
	pending := &DeviceRegistration{
		DisplayName:     name,
		OrgID:           org.ID,
		OrgName:         org.Name,
		DeviceID:        deviceID,
		HardwareProfile: *hwProfile,
		Status:          RegistrationStatusPending,
		RegisteredAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := deps.registrationStore.Save(pending); err != nil {
		return fmt.Errorf("failed to write pending registration: %w", err)
	}

	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	addResp, err := client.AddNode(ctx, connect.NewRequest(&nodev1.AddNodeRequest{
		OrganizationId: org.ID,
		Name:           name,
		DeviceId:       deviceID,
		NodeSpec:       toProtoNodeSpec(hwProfile),
	}))
	if err != nil {
		var connectErr *connect.Error
		if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeAlreadyExists {
			// delete pending registration to prevent stale dupe name
			_ = deps.registrationStore.Delete()
			return errors.New(connectErr.Message())
		}
		return fmt.Errorf("failed to register node: %w", err)
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
		Status:          RegistrationStatusRegistered,
	}

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 4/5] Storing registration data..."))
	if err := deps.registrationStore.Save(reg); err != nil {
		return fmt.Errorf("node registered but failed to save locally: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 5/5] Connecting device to Brev..."))
	runSetup(node, t, deps)

	t.Vprintf("%s  Node registered.\n", t.Green("  ✓"))
	t.Vprintf("%s  Registration complete.\n", t.Green("  ✓"))

	t.Vprint("")
	t.Vprintf("  %s\n", t.Green("To enable SSH access to this device, run: brev enable-ssh"))
	return nil
}

func resolveOrgInteractive(t *terminal.Terminal, s RegisterStore, deps registerDeps) (*entity.Organization, error) {
	list, err := s.ListOrganizations()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	org, err := helpers.SelectOrganizationInteractive(t, list, deps.selector)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return org, nil
}

func resolveOrg(s RegisterStore, orgName string) (*entity.Organization, error) {
	org, err := helpers.ResolveOrgByName(s, orgName)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return org, nil
}

func orgMismatchError(reg *DeviceRegistration, intended *entity.Organization) error {
	existing := "this device is already registered in org"
	if reg.Status == RegistrationStatusPending {
		existing = "an incomplete registration exists for org"
	}
	return breverrors.NewValidationError(fmt.Sprintf(
		"%s %s (%s), not %s (%s); run 'brev deregister' first to register in a different org",
		existing, reg.OrgName, reg.OrgID, intended.Name, intended.ID))
}

func checkExistingRegistration(ctx context.Context, t *terminal.Terminal, s RegisterStore, deps registerDeps, reg *DeviceRegistration) error {
	t.Vprint("")
	t.Vprintf("  This machine is already registered as %s (%s).\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprint("  Checking connectivity...")
	t.Vprint("")

	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	resp, err := client.GetNode(ctx, connect.NewRequest(&nodev1.GetNodeRequest{
		ExternalNodeId: reg.ExternalNodeID,
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

// resumeRegistration reuses the pending record's device ID. AddNode is
// idempotent on device_id, so this recovers when AddNode succeeded backend-side
// but the CLI never confirmed the ExternalNodeID.
func resumeRegistration(ctx context.Context, t *terminal.Terminal, s RegisterStore, deps registerDeps, pending *DeviceRegistration) error {
	t.Vprint("")
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint(t.White("  Resuming incomplete registration"))
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint("")
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Device:")), t.BoldBlue(pending.DisplayName))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Organization:")), t.BoldBlue(pending.OrgName+" ("+pending.OrgID+")"))
	t.Vprintf("  %s %s\n", t.Green(fmt.Sprintf("%-14s", "Device ID:")), t.BoldBlue(pending.DeviceID))
	t.Vprint("")
	t.Vprint("  A previous registration attempt did not finish. Resuming.")

	org := &entity.Organization{ID: pending.OrgID, Name: pending.OrgName}
	return runRegisterSteps(ctx, t, s, pending.DisplayName, org, deps, pending.DeviceID)
}
