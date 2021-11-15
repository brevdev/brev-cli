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

	i := 15
	bar := t.NewProgressBar("Loading...", func() {})
	bar.AdvanceTo(5)
	bar.Describe("Workspace is starting")
	bar.AdvanceTo(i)

	isReady := false
	for isReady != true {
		time.Sleep(1 * time.Second)
		ws, err := client.GetWorkspace(workspace.ID)
		if err != nil {
			// TODO: what do we do here??
		}
		if ws.Status == "RUNNING" {
			bar.Describe("Workspace is ready!")
			bar.AdvanceTo(100)
			isReady = true
		} else {
			if i < 89 {
				i += 2
			} else {
				bar.Describe("Still starting...")
			}
			bar.AdvanceTo(i)
		}
	}

	t.Vprintf(t.Green("\n\nTo connect to your machine, make sure to Brev on:") +
		t.Yellow("\n\t$ brev on\n"))

	return nil

}
