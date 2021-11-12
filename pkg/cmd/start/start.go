// Package start is for starting Brev workspaces
package start

import (
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	startLong    = "Start a Brev machine that's in a paused or off state"
	startExample = "brev start <ws_name>"
)

func NewCmdStart(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "start",
		DisableFlagsInUseLine: true,
		Short:                 "Start a workspace if it's stopped",
		Long:                  startLong,
		Example:               startExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             brev_api.GetCachedWorkspaceNames(),
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

	t.Vprintf(t.Yellow("\nWorkspace %s is starting. \nNote: this can take about a minute. Run 'brev ls' to check status\n\n", startedWorkspace.Name))

	bar := t.NewProgressBar("Loading...", func() {})
	bar.AdvanceTo(1)
	for i := 0; i <= 30; i++ {
		time.Sleep(1 * time.Second)
		bar.AdvanceTo(1+(i*2))
	}

	bar.Describe("Workspace is starting")
	bar.AdvanceTo(62)

	isReady := false
	for isReady != true {
		ws, err := client.GetWorkspace(workspace.ID)
		if err != nil {
			// TODO: what do we do here??
		}
		if ws.Status == "RUNNING" {
			bar.Describe("Workspace is ready!")
			bar.AdvanceTo(100)
			isReady = true
		}
	}


	t.Vprintf(t.Green("\n\nTo connect to your machine, run:")+ 
	t.Yellow("\n\tbrev on "+"brev-cli") )
	
	t.Vprintf(t.Green("\nor access via browser at https://" + "sshghhh.brev.sh\n"))

	return nil

}
