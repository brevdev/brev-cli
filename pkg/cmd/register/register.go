// Package register provides the brev join command and device registration storage.
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

// NetBirdConnector confirms local NetBird management connectivity.
type NetBirdConnector interface {
	EnsureConnected(context.Context) error
}

// NetBirdManager installs, uninstalls, and monitors the NetBird network agent.
type NetBirdManager interface {
	NetBirdConnector
	Install() error
	Uninstall() error
}

// SetupRunner runs a setup script on the local machine.
type SetupRunner interface {
	RunSetup(script string) error
}

type joinPrompter interface {
	terminal.Confirmer
	terminal.Selector
	Input(terminal.PromptContent) string
}

// joinDeps bundles the side-effecting dependencies of runJoin so they
// can be replaced in tests.
type joinDeps struct {
	platform          externalnode.PlatformChecker
	prompter          joinPrompter
	gater             sudo.Gater
	netbird           NetBirdManager
	setupRunner       SetupRunner
	nodeClients       externalnode.NodeClientFactory
	hardwareProfiler  HardwareProfiler
	registrationStore RegistrationStore
}

func defaultJoinDeps() joinDeps {
	p := TerminalPrompter{}
	return joinDeps{
		platform:          LinuxPlatform{},
		prompter:          p,
		gater:             sudo.Default,
		netbird:           Netbird{},
		setupRunner:       ShellSetupRunner{},
		nodeClients:       DefaultNodeClientFactory{},
		hardwareProfiler:  &SystemHardwareProfiler{},
		registrationStore: NewFileRegistrationStore(),
	}
}

var (
	joinLong = `Join this device to a Brev network

This command sets up network connectivity and joins this machine to Brev.

Two modes are supported:
  • Interactive (default): run 'brev join' with no flags and follow prompts for device name and organization.
  • Non-interactive: use --name and --org. No prompts; both are required. Use for scripts/CI.`

	joinExample = `  # Interactive (prompts for device name, organization, and confirmations)
  brev join

  # Non-interactive (any flag implies no prompts; --name and --org required)
  brev join --name my-node --org my-org`
)

func NewCmdJoin(t *terminal.Terminal, store RegisterStore) *cobra.Command {
	return newCmdJoin(t, store, defaultJoinDeps)
}

// NewCmdRegister is retained for source compatibility. It returns the
// canonical join command with register as its deprecated alias.
//
// Deprecated: use NewCmdJoin.
func NewCmdRegister(t *terminal.Terminal, store RegisterStore) *cobra.Command {
	return NewCmdJoin(t, store)
}

func newCmdJoin(t *terminal.Terminal, store RegisterStore, depsFactory func() joinDeps) *cobra.Command {
	var orgFlag string
	var nameFlag string
	var sshPort int
	var approveFlag bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "join",
		Aliases:               []string{"register"},
		DisableFlagsInUseLine: true,
		Short:                 "Join this device to a Brev network",
		Long:                  joinLong,
		Example:               joinExample,
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.CalledAs() == "register" {
				fmt.Fprintln(cmd.ErrOrStderr(), `Warning: "brev register" is deprecated; use "brev join" instead.`)
				fmt.Fprintln(cmd.ErrOrStderr(), `This command no longer enables SSH; run "brev enable-ssh" separately.`)
			}
			if cmd.Flags().Changed("ssh-port") {
				return fmt.Errorf("--ssh-port is no longer supported by brev join or brev register; run brev join, then run brev enable-ssh on the joined machine")
			}
			opts := joinOpts{
				interactive: nameFlag == "" && orgFlag == "",
				name:        nameFlag,
				orgName:     orgFlag,
				skipConfirm: approveFlag,
			}
			return runJoin(cmd.Context(), t, store, opts, depsFactory())
		},
	}

	cmd.Flags().StringVarP(&orgFlag, "org", "o", "", "organization name (required when using non-interactive mode)")
	cmd.Flags().StringVarP(&nameFlag, "name", "n", "", "device name (required when using non-interactive mode)")
	cmd.Flags().IntVarP(&sshPort, "ssh-port", "p", 0, "deprecated")
	_ = cmd.Flags().MarkHidden("ssh-port")
	cmd.Flags().BoolVar(&approveFlag, "approve", false, "skip all confirmation prompts (assume yes)")

	return cmd
}

// joinOpts carries mode and inputs: when interactive, name and orgName are prompted; otherwise they come from flags.
type joinOpts struct {
	interactive bool
	name        string
	orgName     string
	skipConfirm bool
}

// runJoin runs a single membership setup flow; the only difference by mode is whether we prompt or use opts.
func runJoin(ctx context.Context, t *terminal.Terminal, s RegisterStore, opts joinOpts, deps joinDeps) error { //nolint:gocognit,gocyclo,funlen // ok
	// Basic validation
	if !deps.platform.IsCompatible() {
		return breverrors.New("brev join is only supported on Linux")
	}
	// Always gate on sudo; skip confirmation prompt when non-interactive or --approve.
	if err := deps.gater.Gate(t, deps.prompter, "Device join", !opts.interactive || opts.skipConfirm); err != nil {
		return fmt.Errorf("sudo issue: %w", err)
	}
	if !opts.interactive {
		if opts.name == "" || opts.orgName == "" {
			return fmt.Errorf("in non-interactive mode --name and --org are required")
		}
	}

	// Run through the login flow
	_, err := s.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Check if the device is already registered
	alreadyRegistered, err := deps.registrationStore.Exists()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if alreadyRegistered {
		return checkExistingRegistration(ctx, t, s, deps)
	}

	// Capture the device name
	var name string
	if opts.interactive {
		t.Vprint("")
		name = deps.prompter.Input(terminal.PromptContent{
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

	// Capture the target organization
	var org *entity.Organization
	if opts.interactive {
		t.Vprint("")
		org, err = resolveOrgInteractive(t, s, deps)
	} else {
		org, err = resolveOrg(s, opts.orgName)
	}
	if err != nil {
		return err
	}

	t.Vprint("")
	t.Vprint(t.White("══════════════════════════════════════════════════"))
	t.Vprint(t.White("  Joining your device to Brev"))
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
	t.Vprint("    3. Join this machine to Brev")
	t.Vprint("    4. Store join data")
	t.Vprint("    5. Connect device to Brev")
	t.Vprint("")

	if opts.interactive {
		if !opts.skipConfirm && !deps.prompter.ConfirmYesNo("Proceed with join?") {
			t.Vprint("Join canceled.")
			return nil
		}
	}

	if err := runJoinSteps(ctx, t, s, name, org, deps); err != nil {
		return err
	}
	t.Vprint("")
	t.Vprint("SSH access was not enabled. To enable it for your user, run: brev enable-ssh")
	return nil
}

// runJoinSteps performs netbird install, hardware profile, AddNode, save registration, and runSetup.
func runJoinSteps(ctx context.Context, t *terminal.Terminal, s RegisterStore, name string, org *entity.Organization, deps joinDeps) error {
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
	t.Vprint(t.Yellow("[Step 3/5] Joining device to Brev..."))
	deviceID := uuid.New().String()
	client := deps.nodeClients.NewNodeClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	addResp, err := client.AddNode(ctx, connect.NewRequest(&nodev1.AddNodeRequest{
		OrganizationId: org.ID,
		Name:           name,
		DeviceId:       deviceID,
		NodeSpec:       toProtoNodeSpec(hwProfile),
	}))
	if err != nil {
		// dev-plane returns CodeAlreadyExists for a duplicate node name; surface
		// its message directly, which already reads as "node already exists".
		var connectErr *connect.Error
		if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeAlreadyExists {
			return errors.New(connectErr.Message())
		}
		return fmt.Errorf("failed to join node: %w", err)
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

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 4/5] Storing registration data..."))
	if err := deps.registrationStore.Save(reg); err != nil {
		return fmt.Errorf("node joined but failed to save locally: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 5/5] Connecting device to Brev..."))
	runSetup(node, t, deps)

	t.Vprintf("%s  Node joined.\n", t.Green("  ✓"))
	t.Vprintf("%s  Join complete.\n", t.Green("  ✓"))
	return nil
}

func resolveOrgInteractive(t *terminal.Terminal, s RegisterStore, deps joinDeps) (*entity.Organization, error) {
	list, err := s.ListOrganizations()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	org, err := helpers.SelectOrganizationInteractive(t, list, deps.prompter)
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

// checkExistingRegistration verifies connectivity for an already-registered node.
// It calls GetNode to check the server-side NetworkMemberStatus and ensures the
// local netbird service is running, starting it if necessary. Returns nil if
// the node is healthy, or an error describing what's wrong.
func checkExistingRegistration(ctx context.Context, t *terminal.Terminal, s RegisterStore, deps joinDeps) error {
	reg, loadErr := deps.registrationStore.Load()
	if loadErr != nil {
		return fmt.Errorf("this machine is already registered but the registration file could not be read: %w", loadErr)
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
		} else {
			t.Vprintf("  Node status: %s\n", externalnode.FriendlyNetworkStatus(ci.GetStatus()))
		}
	}

	// Confirm local NetBird connectivity even when the backend is connected.
	t.Vprint("  Checking local Brev tunnel...")
	if err := deps.netbird.EnsureConnected(ctx); err != nil {
		t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: %v", err)))
	} else {
		t.Vprint(t.Green("  Brev tunnel is connected."))
	}

	t.Vprint("")
	t.Vprint("  Run 'brev leave' first if you want to rejoin.")
	return nil
}

func runSetup(node *nodev1.ExternalNode, t *terminal.Terminal, deps joinDeps) {
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
