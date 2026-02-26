// Package register provides the brev register command for DGX Spark registration
package register

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"time"

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

var (
	registerLong = `Register your DGX Spark with NVIDIA Brev

This command installs NetBird (network agent), and registers this machine with Brev.`

	registerExample = `  brev register --name "My DGX Spark"`
)

func NewCmdRegister(t *terminal.Terminal, store RegisterStore) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "register",
		Aliases:               []string{"spark"},
		DisableFlagsInUseLine: true,
		Short:                 "Register this device with Brev",
		Long:                  registerLong,
		Example:               registerExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegister(cmd.Context(), t, store, name)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Display name (required)")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func runRegister(ctx context.Context, t *terminal.Terminal, s RegisterStore, name string) error { //nolint:funlen // registration flow
	if runtime.GOOS != "linux" {
		return fmt.Errorf("brev register is only supported on Linux (DGX Spark)")
	}

	currentUser, err := s.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	org, err := s.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return fmt.Errorf("no organization found; please create or join an organization first")
	}

	brevHome, err := s.GetBrevHomePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if RegistrationExists(brevHome) {
		return fmt.Errorf("this machine is already registered; run 'brev deregister' first to re-register")
	}

	linuxUser := currentUser.Username
	if u, err := user.Current(); err == nil {
		linuxUser = u.Username
	}

	t.Vprint("")
	t.Vprint(t.Green("Registering your DGX Spark with Brev"))
	t.Vprint("")
	t.Vprintf("  Name:         %s\n", t.Yellow(name))
	t.Vprintf("  Organization: %s\n", org.Name)
	t.Vprintf("  Linux user:   %s\n", linuxUser)
	t.Vprint("")
	t.Vprint("This will perform the following steps:")
	t.Vprint("  1. Install NetBird (network agent)")
	t.Vprint("  2. Collect hardware profile")
	t.Vprint("  3. Register this machine with Brev")
	t.Vprint("")

	result := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label: "Proceed with registration?",
		Items: []string{"Yes, proceed", "No, cancel"},
	})
	if result != "Yes, proceed" {
		t.Vprint("Registration canceled.")
		return nil
	}

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 1/3] Installing NetBird..."))
	if err := InstallNetbird(t); err != nil {
		return fmt.Errorf("NetBird installation failed: %w", err)
	}
	t.Vprint(t.Green("  NetBird installed successfully."))

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 2/3] Collecting hardware profile..."))
	t.Vprint("")

	runner := ExecCommandRunner{}
	reader := OSFileReader{}
	nodeSpec, err := CollectHardwareProfile(runner, reader)
	if err != nil {
		return fmt.Errorf("failed to collect hardware profile: %w", err)
	}

	t.Vprint("  Hardware profile:")
	t.Vprint(FormatNodeSpec(nodeSpec))

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 3/3] Registering with Brev..."))

	deviceID := uuid.New().String()
	client := NewNodeServiceClient(s, config.GlobalConfig.GetBrevAPIURl())
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
	if err := SaveRegistration(brevHome, reg); err != nil {
		return fmt.Errorf("node registered but failed to save locally: %w", err)
	}

	t.Vprint(t.Green("  Registration complete."))

	if cmd := addResp.Msg.GetSetupCommand(); cmd != "" {
		if err := runSetupCommand(cmd); err != nil {
			t.Vprintf("  Warning: setup command failed: %v\n", err)
		}
	}
	return nil
}
