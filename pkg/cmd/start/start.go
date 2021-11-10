// Package start is for starting Brev workspaces
package start

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	startLong    = "Start a Brev machine that's in a paused or off state"
	startExample = "brev start <ws_name>"
)

func getCachedWorkspaceNames() []string {
	activeOrg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		return nil
	}

	cachedWorkspaces, err := brev_api.GetWsCacheData()
	if err != nil {
		return nil
	}

	var wsNames []string
	for _, cw := range cachedWorkspaces {
		if cw.OrgID == activeOrg.ID {
			for _, w := range cw.Workspaces {
				wsNames = append(wsNames, w.Name)
			}
			return wsNames
		}
	}

	return nil
}

func NewCmdStart(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "start",
		DisableFlagsInUseLine: true,
		Short:                 "Start a workspace if it's stopped",
		Long:                  startLong,
		Example:               startExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             getCachedWorkspaceNames(),
		Run: func(cmd *cobra.Command, args []string) {

			err := startWorkspace(args[0], t)
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}

		},
	}

	return cmd
}

func startWorkspace(workspaceName string, t *terminal.Terminal) error {
	client, err := brev_api.NewCommandClient()
	if err != nil {
		return err
	}

	workspace, err := brev_api.GetWorkspaceFromName(workspaceName)
	if err != nil {
		return err
	}

	startedWorkspace, err := client.StartWorkspace(workspace.ID)
	if err != nil {
		fmt.Println(err)
		return err
	}

	t.Vprintf("Workspace %s is starting. \n Note: this can take a few seconds. Run 'brev ls' to check status", startedWorkspace.Name)

	return nil

}
