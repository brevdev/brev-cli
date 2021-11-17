// Package stop is for stopping Brev workspaces
package stop

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	stopLong    = "Stop a Brev machine that's in a running state"
	stopExample = "brev stop <ws_name>"
)

func getWorkspaceNames() []string {
	activeOrg, err := brevapi.GetActiveOrgContext(files.AppFs)
	if err != nil {
		return nil
	}

	client, err := brevapi.NewCommandClient()
	if err != nil {
		return nil
	}
	wss, err := client.GetMyWorkspaces(activeOrg.ID)
	if err != nil {
		return nil
	}

	var wsNames []string
	for _, w := range wss {
		wsNames = append(wsNames, w.Name)
	}

	return wsNames
}

type StopStore interface {
	completions.CompletionStore
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]brevapi.Workspace, error)
	StopWorkspace(workspaceID string) (*brevapi.Workspace, error)
}

func NewCmdStop(stopStore StopStore, t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "stop",
		DisableFlagsInUseLine: true,
		Short:                 "Stop a workspace if it's running",
		Long:                  stopLong,
		Example:               stopExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             getWorkspaceNames(),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(stopStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			err := stopWorkspace(args[0], t, stopStore)
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}
		},
	}

	return cmd
}

func stopWorkspace(workspaceName string, t *terminal.Terminal, stopStore StopStore) error {
	var workspaces []brevapi.Workspace
	org, err := stopStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		workspaces, err = stopStore.GetAllWorkspaces(nil)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		workspaces, err = stopStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{Name: workspaceName})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	var workspace brevapi.Workspace
	if len(workspaces) == 0 {
		return fmt.Errorf("no workspaces found with name %s", workspaceName)
	} else if len(workspaces) > 1 {
		return fmt.Errorf("multiple workspaces found with name %s", workspaceName)
	} else {
		workspace = workspaces[0]
	}

	_, err = stopStore.StopWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf(t.Green("Workspace "+workspace.Name+" is stopping.") +
		"\nNote: this can take a few seconds. Run 'brev ls' to check status\n")

	return nil
}
