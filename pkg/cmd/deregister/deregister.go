// Package deregister provides the brev deregister command for DGX Spark deregistration
package deregister

import (
	"context"
	"fmt"
	"runtime"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// DeregisterStore defines the store methods needed by the deregister command.
type DeregisterStore interface {
	GetCurrentUser() (*entity.User, error)
	GetBrevHomePath() (string, error)
	GetAccessToken() (string, error)
}

var (
	deregisterLong = `Deregister your DGX Spark from NVIDIA Brev

This command removes the local registration data and optionally uninstalls
NetBird (network agent).`

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
			return runDeregister(cmd.Context(), t, store)
		},
	}

	return cmd
}

func runDeregister(ctx context.Context, t *terminal.Terminal, s DeregisterStore) error { //nolint:funlen // deregistration flow
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
	t.Vprintf("  Node ID: %s\n", reg.ExternalNodeID)
	t.Vprintf("  Name:    %s\n", reg.DisplayName)
	t.Vprint("")

	removeNetbird := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label: "Would you also like to uninstall NetBird?",
		Items: []string{"Yes, uninstall NetBird", "No, keep NetBird installed"},
	})

	confirm := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label: "Proceed with deregistration?",
		Items: []string{"Yes, proceed", "No, cancel"},
	})
	if confirm != "Yes, proceed" {
		t.Vprint("Deregistration canceled.")
		return nil
	}

	t.Vprint("")
	t.Vprint(t.Yellow("Removing node from Brev..."))
	client := register.NewConnectNodeClient(s, config.GlobalConfig.GetBrevAPIURl())
	if err := client.RemoveNode(ctx, &register.RemoveNodeRequest{
		ExternalNodeID: reg.ExternalNodeID,
		OrganizationID: reg.OrgID,
	}); err != nil {
		return fmt.Errorf("failed to deregister node: %w", err)
	}
	t.Vprint(t.Green("  Node removed from Brev."))
	t.Vprint("")

	if removeNetbird == "Yes, uninstall NetBird" {
		t.Vprint("Removing NetBird...")
		if err := register.UninstallNetbird(t); err != nil {
			t.Vprintf("  Warning: failed to uninstall NetBird: %v\n", err)
		} else {
			t.Vprint(t.Green("  NetBird uninstalled."))
		}
		t.Vprint("")
	}

	t.Vprint("Removing registration data...")
	if err := register.DeleteRegistration(brevHome); err != nil {
		return fmt.Errorf("failed to remove registration data: %w", err)
	}

	t.Vprint(t.Green("Deregistration complete."))
	t.Vprint("")

	return nil
}
