// Package register provides the brev register command for device registration
package register

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"time"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
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

// PlatformChecker checks whether the current platform is supported.
type PlatformChecker interface {
	IsCompatible() bool
}

// Confirmer prompts for yes/no confirmation.
type Confirmer interface {
	ConfirmYesNo(label string) bool
}

// NetBirdInstaller installs the NetBird network agent.
type NetBirdInstaller interface {
	Install() error
}

// SetupRunner runs a setup script on the local machine.
type SetupRunner interface {
	RunSetup(script string) error
}

// NodeClientFactory creates ConnectRPC ExternalNodeService clients.
type NodeClientFactory interface {
	NewNodeClient(provider TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient
}

// registerDeps bundles the side-effecting dependencies of runRegister so they
// can be replaced in tests.
type registerDeps struct {
	platform          PlatformChecker
	prompter          Confirmer
	netbird           NetBirdInstaller
	setupRunner       SetupRunner
	nodeClients       NodeClientFactory
	commandRunner     CommandRunner
	fileReader        FileReader
	registrationStore RegistrationStore
}

func defaultRegisterDeps(brevHome string) registerDeps {
	return registerDeps{
		platform:          LinuxPlatform{},
		prompter:          TerminalPrompter{},
		netbird:           NetBirdManager{},
		setupRunner:       ShellSetupRunner{},
		nodeClients:       DefaultNodeClientFactory{},
		commandRunner:     ExecCommandRunner{},
		fileReader:        OSFileReader{},
		registrationStore: NewFileRegistrationStore(brevHome),
	}
}

var (
	registerLong = `Register your device with NVIDIA Brev

This command installs NetBird (network agent), and registers this machine with Brev.`

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
	org, err := getOrgToRegisterFor(deps, s)
	if err != nil {
		return err
	}

	brevUser, err := s.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to determine current Linux user: %w", err)
	}
	linuxUser := u.Username

	t.Vprint("")
	t.Vprint(t.Green("Registering your device with Brev"))
	t.Vprint("")
	t.Vprintf("  Name:         %s\n", t.Yellow(name))
	t.Vprintf("  Organization: %s\n", org.Name)
	t.Vprintf("  Registering for Linux user:   %s\n", linuxUser)
	t.Vprint("")
	t.Vprint("This will perform the following steps:")
	t.Vprint("  1. Install NetBird")
	t.Vprint("  2. Collect hardware profile")
	t.Vprint("  3. Register this machine with Brev")
	t.Vprint("")

	if !deps.prompter.ConfirmYesNo("Proceed with registration?") {
		t.Vprint("Registration canceled.")
		return nil
	}

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 1/3] Installing NetBird..."))
	if err := deps.netbird.Install(); err != nil {
		return fmt.Errorf("NetBird installation failed: %w", err)
	}
	t.Vprint(t.Green("  NetBird installed successfully."))

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

	if ci := node.GetConnectivityInfo(); ci != nil {
		if cmd := ci.GetRegistrationCommand(); cmd != "" {
			if err := deps.setupRunner.RunSetup(cmd); err != nil {
				t.Vprintf("  Warning: setup command failed: %v\n", err)
			}
		}
	}

	if deps.prompter.ConfirmYesNo("Would you like to enable SSH access to this device?") {
		grantSSHAccess(ctx, t, deps, s, reg, brevUser, u)
	}

	return nil
}

func grantSSHAccess(ctx context.Context, t *terminal.Terminal, deps registerDeps, tokenProvider TokenProvider, reg *DeviceRegistration, brevUser *entity.User, u *user.User) {
	t.Vprint("")
	t.Vprint(t.Green("Enabling SSH access on this device"))
	t.Vprint("")
	t.Vprintf("  Node:       %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
	t.Vprintf("  Brev user:  %s\n", brevUser.ID)
	t.Vprintf("  Linux user: %s\n", u.Username)
	t.Vprint("")

	client := deps.nodeClients.NewNodeClient(tokenProvider, config.GlobalConfig.GetBrevPublicAPIURL())
	if _, err := client.GrantNodeSSHAccess(ctx, connect.NewRequest(&nodev1.GrantNodeSSHAccessRequest{
		ExternalNodeId: reg.ExternalNodeID,
		UserId:         brevUser.ID,
		LinuxUser:      u.Username,
	})); err != nil {
		t.Vprintf("  Warning: failed to enable SSH: %v\n", err)
		return
	}

	if brevUser.PublicKey != "" {
		if err := InstallAuthorizedKey(u, brevUser.PublicKey); err != nil {
			t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: failed to install SSH public key: %v", err)))
		} else {
			t.Vprint("  Brev public key added to authorized_keys.")
		}
	}

	t.Vprint(t.Green(fmt.Sprintf("SSH access enabled. You can now SSH to this device via: brev shell %s", reg.DisplayName)))
}

func getOrgToRegisterFor(deps registerDeps, s RegisterStore) (*entity.Organization, error) {
	if !deps.platform.IsCompatible() {
		return nil, fmt.Errorf("brev register is only supported on Linux")
	}

	_, err := s.GetCurrentUser() // ensure active token
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	org, err := s.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, fmt.Errorf("no organization found; please create or join an organization first")
	}

	alreadyRegistered, err := deps.registrationStore.Exists()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if alreadyRegistered {
		return nil, fmt.Errorf("this machine is already registered; run 'brev deregister' first to re-register")
	}
	return org, nil
}
