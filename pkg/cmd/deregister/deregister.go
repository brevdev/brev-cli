// Package deregister provides the brev deregister command for device deregistration
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
	goos              string
	promptSelect      func(label string, items []string) string
	uninstallNetbird  func() error
	newNodeClient     func(provider register.TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient
	registrationStore register.RegistrationStore
}

func prodDeregisterDeps(brevHome string) deregisterDeps {
	return deregisterDeps{
		goos: runtime.GOOS,
		promptSelect: func(label string, items []string) string {
			return terminal.PromptSelectInput(terminal.PromptSelectContent{
				Label: label,
				Items: items,
			})
		},
		uninstallNetbird:  register.UninstallNetbird,
		newNodeClient:     register.NewNodeServiceClient,
		registrationStore: register.NewFileRegistrationStore(brevHome),
	}
}

var (
	deregisterLong = `Deregister your device from NVIDIA Brev

This command removes the local registration data and optionally uninstalls
NetBird (network agent).`

	deregisterExample = `  brev deregister`
)

func NewCmdDeregister(t *terminal.Terminal, store DeregisterStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "deregister",
		DisableFlagsInUseLine: true,
		Short:                 "Deregister your device from Brev",
		Long:                  deregisterLong,
		Example:               deregisterExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			brevHome, err := store.GetBrevHomePath()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return runDeregister(cmd.Context(), t, store, prodDeregisterDeps(brevHome))
		},
	}

	return cmd
}

func runDeregister(ctx context.Context, t *terminal.Terminal, s DeregisterStore, deps deregisterDeps) error { //nolint:funlen // deregistration flow
	if deps.goos != "linux" {
		return fmt.Errorf("brev deregister is only supported on Linux")
	}

	registered, err := deps.registrationStore.Exists()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !registered {
		return fmt.Errorf("no registration found; this machine does not appear to be registered\nRun 'brev register' to register your device")
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return fmt.Errorf("failed to read registration file: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green("Deregistering device"))
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
		if err := deps.uninstallNetbird(); err != nil {
			t.Vprintf("  Warning: failed to uninstall NetBird: %v\n", err)
		} else {
			t.Vprint(t.Green("  NetBird uninstalled."))
		}
		t.Vprint("")
	}

	t.Vprint("Removing registration data...")
	if err := deps.registrationStore.Delete(); err != nil {
		t.Vprintf("  Warning: failed to remove local registration file: %v\n", err)
		t.Vprint("  You can manually remove it with: rm ~/.brev/device_registration.json")
	}

	t.Vprint(t.Green("Deregistration complete."))
	t.Vprint("")

	return nil
}
