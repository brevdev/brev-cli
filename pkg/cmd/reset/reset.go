// Package start is for starting Brev workspaces
package reset

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	startLong    = "Reset your machine if it's acting up. This deletes the machine and gets you a fresh one."
	startExample = "brev reset <ws_name>"
)

func NewCmdReset(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "reset",
		DisableFlagsInUseLine: true,
		Short:                 "Reset a workspace if it's in a weird state.",
		Long:                  startLong,
		Example:               startExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             brevapi.GetCachedWorkspaceNames(),
		Run: func(cmd *cobra.Command, args []string) {
			err := resetWorkspace(args[0], t)
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}
		},
	}

	return cmd
}

func resetWorkspace(workspaceName string, t *terminal.Terminal) error {
	client, err := brevapi.NewCommandClient()
	if err != nil {
		return err
	}

	workspace, err := brevapi.GetWorkspaceFromName(workspaceName)
	if err != nil {
		return err
	}

	startedWorkspace, err := client.ResetWorkspace(workspace.ID)
	if err != nil {
		fmt.Println(err)
		return err
	}

	t.Vprintf("Workspace %s is resetting. \n Note: this can take a few seconds. Run 'brev ls' to check status", startedWorkspace.Name)

	return nil
}
