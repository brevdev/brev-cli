// Package start is for starting Brev workspaces
package start

import (
	"fmt"
	"time"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"

	"github.com/brevdev/brev-cli/pkg/brevapi"
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
		ValidArgs:             brevapi.GetCachedWorkspaceNames(),
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
	client, err := brevapi.NewCommandClient()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspace, err := brevapi.GetWorkspaceFromName(workspaceName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	startedWorkspace, err := client.StartWorkspace(workspace.ID)
	if err != nil {
		fmt.Println(err)
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf(t.Yellow("\nWorkspace %s is starting. \nNote: this can take about a minute. Run 'brev ls' to check status\n\n", startedWorkspace.Name))

	loadingAdv := 5
	bar := t.NewProgressBar("Loading...", func() {})
	bar.AdvanceTo(loadingAdv)
	bar.Describe("Workspace is starting")
	startingBar := 15
	bar.AdvanceTo(startingBar)

	total := 100
	stdIncrement := 1
	startingIncrement := 2
	startingIncrementThresh := 89

	isReady := false
	for !isReady {
		time.Sleep(time.Duration(stdIncrement) * time.Second)
		ws, err := client.GetWorkspace(workspace.ID)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if ws.Status == "RUNNING" {
			bar.Describe("Workspace is ready!")
			bar.AdvanceTo(total)
			isReady = true
		} else {
			if startingBar < startingIncrementThresh {
				startingBar += startingIncrement
			} else {
				bar.Describe("Still starting...")
			}
			bar.AdvanceTo(startingBar)
		}
	}

	t.Vprintf(t.Green("\n\nTo connect to your machine, make sure to Brev on:") +
		t.Yellow("\n\t$ brev on\n"))

	return nil
}
