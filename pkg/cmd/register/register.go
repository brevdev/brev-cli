// Package register provides the brev register command for device registration
package register

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"time"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// RegisterStore defines the store methods needed by the register command.
type RegisterStore interface {
	GetCurrentUser() (*entity.User, error)
	GetCurrentUserKeys() (*entity.UserKeys, error)
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

// registerDeps bundles the side-effecting dependencies of runRegister so they
// can be replaced in tests.
type registerDeps struct {
	goos              string
	promptYesNo       func(label string) bool
	installNetbird    func() error
	runSetupCommand   func(script string) error
	newNodeClient     func(provider TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient
	commandRunner     CommandRunner
	fileReader        FileReader
	registrationStore RegistrationStore
}

func prodRegisterDeps(brevHome string) registerDeps {
	return registerDeps{
		goos: runtime.GOOS,
		promptYesNo: func(label string) bool {
			result := terminal.PromptSelectInput(terminal.PromptSelectContent{
				Label: label,
				Items: []string{"Yes, proceed", "No, cancel"},
			})
			return result == "Yes, proceed"
		},
		installNetbird:    InstallNetbird,
		runSetupCommand:   runSetupCommand,
		newNodeClient:     NewNodeServiceClient,
		commandRunner:     ExecCommandRunner{},
		fileReader:        OSFileReader{},
		registrationStore: NewFileRegistrationStore(brevHome),
	}
}

var (
	registerLong = `Register your device with NVIDIA Brev

This command installs NetBird (network agent), and registers this machine with Brev.`

	registerExample = `  brev register --name "My DGX Spark"`
)

func NewCmdRegister(t *terminal.Terminal, store RegisterStore) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "register",
		DisableFlagsInUseLine: true,
		Short:                 "Register this device with Brev",
		Long:                  registerLong,
		Example:               registerExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			brevHome, err := store.GetBrevHomePath()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return runRegister(cmd.Context(), t, store, name, prodRegisterDeps(brevHome))
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Display name (required)")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func runRegister(ctx context.Context, t *terminal.Terminal, s RegisterStore, name string, deps registerDeps) error { //nolint:funlen // registration flow
	org, err := getOrgToRegisterFor(deps, s)
	if err != nil {
		return err
	}

	u, _ := user.Current()
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

	if !deps.promptYesNo("Proceed with registration?") {
		t.Vprint("Registration canceled.")
		return nil
	}

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 1/3] Installing NetBird..."))
	if err := deps.installNetbird(); err != nil {
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
	client := deps.newNodeClient(s, config.GlobalConfig.GetBrevAPIURl())
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

	// Add user's public key to authorized_keys for SSH access
	keys, err := s.GetCurrentUserKeys()
	if err != nil {
		t.Vprintf("  Warning: failed to get user keys: %v\n", err)
	} else if keys.PublicKey != "" {
		if err := files.WriteAuthorizedKey(files.AppFs, keys.PublicKey, u.HomeDir); err != nil {
			t.Vprintf("  Warning: failed to add public key to authorized_keys: %v\n", err)
		} else {
			t.Vprint(t.Green("  Added public key to authorized_keys."))
		}
	}

	if cmd := addResp.Msg.GetSetupCommand(); cmd != "" {
		if err := deps.runSetupCommand(cmd); err != nil {
			t.Vprintf("  Warning: setup command failed: %v\n", err)
		}
	}
	return nil
}

func getOrgToRegisterFor(deps registerDeps, s RegisterStore) (*entity.Organization, error) {
	if deps.goos != "linux" {
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
