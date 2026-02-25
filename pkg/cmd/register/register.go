// Package register provides the brev register command for DGX Spark registration
package register

import (
	"fmt"
	"os/user"
	"runtime"

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
}

// OSFileReader reads files from the real OS filesystem.
type OSFileReader struct{}

func (r OSFileReader) ReadFile(path string) ([]byte, error) {
	return readOSFile(path)
}

var (
	registerLong = `Register your DGX Spark with NVIDIA Brev

This command installs netbird (network agent), collects a hardware profile,
and registers this machine with Brev.`

	registerExample = `  brev register --name "My DGX Spark"`
)

func NewCmdRegister(t *terminal.Terminal, store RegisterStore) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "register",
		Aliases:               []string{"spark"},
		DisableFlagsInUseLine: true,
		Short:                 "Register your DGX Spark with Brev",
		Long:                  registerLong,
		Example:               registerExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegister(t, store, name)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Display name for this DGX Spark (required)")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func runRegister(t *terminal.Terminal, s RegisterStore, name string) error { //nolint:funlen // registration flow
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
	t.Vprint("  1. Install netbird (network agent)")
	t.Vprint("  2. Register this machine with Brev")
	t.Vprint("")

	result := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label: "Proceed with registration?",
		Items: []string{"Yes, proceed", "No, cancel"},
	})
	if result != "Yes, proceed" {
		t.Vprint("Registration cancelled.")
		return nil
	}

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 1/2] Installing netbird..."))
	if err := InstallNetbird(t); err != nil {
		return fmt.Errorf("netbird installation failed: %w", err)
	}
	t.Vprint(t.Green("  Netbird installed successfully."))

	t.Vprint("")
	t.Vprint(t.Yellow("[Step 2/2] Collecting hardware profile..."))
	t.Vprint("")

	runner := ExecCommandRunner{}
	reader := OSFileReader{}
	profile, err := CollectHardwareProfile(runner, reader)
	if err != nil {
		return fmt.Errorf("failed to collect hardware profile: %w", err)
	}

	t.Vprint("  Hardware profile:")
	t.Vprint(FormatHardwareProfile(profile))

	desc := HardwareProfileToDescriptor(profile)
	fingerprint, err := ComputeHardwareFingerprint(desc)
	if err != nil {
		t.Vprintf("  Warning: could not compute hardware fingerprint: %v\n", err)
	} else {
		t.Vprintf("  Fingerprint: %s\n", fingerprint)
	}

	t.Vprint("")
	t.Vprint(t.Yellow("[TODO] Registration API call not yet implemented."))
	t.Vprint("       Once implemented, the backend will return a node ID")
	t.Vprintf("       that will be persisted to %s/spark_registration.json.\n", brevHome)
	t.Vprint("")

	_ = org.ID // will be used in the registration API call
	_ = name   // will be sent as display_name

	return nil
}
