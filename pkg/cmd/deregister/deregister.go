// Package deregister provides the brev deregister command for DGX Spark deregistration
package deregister

import (
	"context"
	"fmt"
	"runtime"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

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

// deregisterDeps bundles the side-effecting dependencies of runDeregister so
// they can be replaced in tests.
type deregisterDeps struct {
	goos               string
	promptSelect       func(label string, items []string) string
	uninstallNetbird   func(t *terminal.Terminal) error
	newNodeClient      func(provider register.TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient
	registrationExists func(brevHome string) (bool, error)
	loadRegistration   func(brevHome string) (*register.DeviceRegistration, error)
	deleteRegistration func(brevHome string) error
}

func prodDeregisterDeps() deregisterDeps {
	return deregisterDeps{
		goos: runtime.GOOS,
		promptSelect: func(label string, items []string) string {
			return terminal.PromptSelectInput(terminal.PromptSelectContent{
				Label: label,
				Items: items,
			})
		},
		uninstallNetbird:   register.UninstallNetbird,
		newNodeClient:      register.NewNodeServiceClient,
		registrationExists: register.RegistrationExists,
		loadRegistration:   register.LoadRegistration,
		deleteRegistration: register.DeleteRegistration,
	}
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
			return runDeregister(cmd.Context(), t, store, prodDeregisterDeps())
		},
	}

	return cmd
}

func runDeregister(ctx context.Context, t *terminal.Terminal, s DeregisterStore, deps deregisterDeps) error { //nolint:funlen // deregistration flow
	if deps.goos != "linux" {
		return fmt.Errorf("brev deregister is only supported on Linux (DGX Spark)")
	}

	brevHome, err := s.GetBrevHomePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	registered, err := deps.registrationExists(brevHome)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !registered {
		return fmt.Errorf("no registration found; this machine does not appear to be registered\nRun 'brev register' to register your DGX Spark")
	}

	reg, err := deps.loadRegistration(brevHome)
	if err != nil {
		return fmt.Errorf("failed to read registration file: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green("Deregistering DGX Spark"))
	t.Vprint("")
	t.Vprintf("  Node ID: %s\n", reg.ExternalNodeID)
	t.Vprintf("  Name:    %s\n", reg.DisplayName)
	t.Vprint("")

	removeNetbird := deps.promptSelect(
		"Would you also like to uninstall NetBird?",
		[]string{"Yes, uninstall NetBird", "No, keep NetBird installed"},
	)

	confirm := deps.promptSelect(
		"Proceed with deregistration?",
		[]string{"Yes, proceed", "No, cancel"},
	)
	if confirm != "Yes, proceed" {
		t.Vprint("Deregistration canceled.")
		return nil
	}

	t.Vprint("")
	t.Vprint(t.Yellow("Removing node from Brev..."))
	client := deps.newNodeClient(s, config.GlobalConfig.GetBrevAPIURl())
	if _, err := client.RemoveNode(ctx, connect.NewRequest(&nodev1.RemoveNodeRequest{
		ExternalNodeId: reg.ExternalNodeID,
		OrganizationId: reg.OrgID,
	})); err != nil {
		return fmt.Errorf("failed to deregister node: %w", err)
	}
	t.Vprint(t.Green("  Node removed from Brev."))
	t.Vprint("")

	if removeNetbird == "Yes, uninstall NetBird" {
		t.Vprint("Removing NetBird...")
		if err := deps.uninstallNetbird(t); err != nil {
			t.Vprintf("  Warning: failed to uninstall NetBird: %v\n", err)
		} else {
			t.Vprint(t.Green("  NetBird uninstalled."))
		}
		t.Vprint("")
	}

	t.Vprint("Removing registration data...")
	if err := deps.deleteRegistration(brevHome); err != nil {
		t.Vprintf("  Warning: failed to remove local registration file: %v\n", err)
		t.Vprint("  You can manually remove it with: rm ~/.brev/spark_registration.json")
	}

	t.Vprint(t.Green("Deregistration complete."))
	t.Vprint("")

	return nil
}
