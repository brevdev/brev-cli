// Package deregister provides the brev deregister command for DGX Spark deregistration
package deregister

import (
	"fmt"
	"runtime"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// DeregisterStore defines the store methods needed by the deregister command.
type DeregisterStore interface {
	GetCurrentUser() (*entity.User, error)
	GetBrevHomePath() (string, error)
}

var (
	deregisterLong = `Deregister your DGX Spark from NVIDIA Brev

This command removes the local registration data and optionally uninstalls
netbird (network agent).`

	deregisterExample = `  brev deregister`
)

func NewCmdDeregister(t *terminal.Terminal, store DeregisterStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "deregister",
		DisableFlagsInUseLine: true,
		Short:                 "Deregister your DGX Spark from Brev",
		Long:                  deregisterLong,
		Example:               deregisterExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeregister(t, store)
		},
	}

	return cmd
}

func runDeregister(t *terminal.Terminal, s DeregisterStore) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("brev deregister is only supported on Linux (DGX Spark)")
	}

	brevHome, err := s.GetBrevHomePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if !register.RegistrationExists(brevHome) {
		return fmt.Errorf("no registration found; this machine does not appear to be registered\nRun 'brev register' to register your DGX Spark")
	}

	reg, err := register.LoadRegistration(brevHome)
	if err != nil {
		return fmt.Errorf("failed to read registration file: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green("Deregistering DGX Spark"))
	t.Vprint("")
	t.Vprintf("  Node ID: %s\n", reg.BrevCloudNodeID)
	t.Vprintf("  Name:    %s\n", reg.DisplayName)
	t.Vprint("")

	removeNetbird := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label: "Would you also like to uninstall netbird?",
		Items: []string{"Yes, uninstall netbird", "No, keep netbird installed"},
	})

	confirm := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label: "Proceed with deregistration?",
		Items: []string{"Yes, proceed", "No, cancel"},
	})
	if confirm != "Yes, proceed" {
		t.Vprint("Deregistration cancelled.")
		return nil
	}

	t.Vprint("")
	t.Vprint(t.Yellow("[TODO] Deregistration API call not yet implemented."))
	t.Vprint("")

	if removeNetbird == "Yes, uninstall netbird" {
		t.Vprint("Removing netbird...")
		if err := register.UninstallNetbird(t); err != nil {
			t.Vprintf("  Warning: failed to uninstall netbird: %v\n", err)
		} else {
			t.Vprint(t.Green("  Netbird uninstalled."))
		}
		t.Vprint("")
	}

	t.Vprint("Removing local registration data...")
	if err := register.DeleteRegistration(brevHome); err != nil {
		return fmt.Errorf("failed to remove registration data: %w", err)
	}

	t.Vprint(t.Green("Deregistration complete."))
	t.Vprint("")

	return nil
}
