// notebook/notebook.go
package notebook

import (
	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	"github.com/brevdev/brev-cli/pkg/cmd/portforward"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	notebookLong    = "Open a notebook on your Brev machine"
	notebookExample = "brev notebook <InstanceName>"
)

type NotebookStore interface {
	portforward.PortforwardStore
}

func NewCmdNotebook(store NotebookStore, _ *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "notebook",
		Short:   "Open a notebook on your Brev machine",
		Long:    notebookLong,
		Example: notebookExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Type out the checking message
			hello.TypeItToMeUnskippable("Checking to make sure the workspace is running...")

			// Validate if the workspace exists
			workspace, err := util.GetUserWorkspaceByNameOrIDErr(store, args[0])
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			// Check if the workspace is running
			if workspace.Status != "RUNNING" {
				hello.TypeItToMeUnskippable("The workspace is not running. Please ensure it's in the running state before proceeding.")
				return breverrors.WorkspaceNotRunning{Status: workspace.Status}
			}

			urlType := color.New(color.FgCyan, color.Bold).SprintFunc()
			warningType := color.New(color.FgBlack, color.Bold, color.BgCyan).SprintFunc()

			hello.TypeItToMeUnskippable("\n" + warningType("  Please keep this terminal open 🤙  "))

			hello.TypeItToMeUnskippable("\nClick here to go to your Jupyter notebook:\n\t 👉" + urlType("http://localhost:8888") + "👈\n\n\n")

			// Port forward on 8888
			err2 := portforward.RunPortforward(store, args[0], "8888:8888")
			if err2 != nil {
				return breverrors.WrapAndTrace(err2)
			}

			// Print out a link for the user
			hello.TypeItToMeUnskippable("Your notebook is accessible at: http://localhost:8888")

			return nil
		},
	}

	return cmd
}
