// Package stop is for stopping Brev workspaces
package stop

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
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

func NewCmdStop(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "stop",
		DisableFlagsInUseLine: true,
		Short:                 "Stop a workspace if it's running",
		Long:                  stopLong,
		Example:               stopExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             getWorkspaceNames(),
		Run: func(cmd *cobra.Command, args []string) {
			err := stopWorkspace(args[0], t)
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}
		},
	}

	return cmd
}

func stopWorkspace(workspaceName string, t *terminal.Terminal) error {
	client, err := brevapi.NewCommandClient()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspace, err := brevapi.GetWorkspaceFromName(workspaceName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	startedWorkspace, err := client.StopWorkspace(workspace.ID)
	if err != nil {
		fmt.Println(err)
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf(t.Green("Workspace "+startedWorkspace.Name+" is stopping.") +
		"\nNote: this can take a few seconds. Run 'brev ls' to check status\n")

	return nil
}
