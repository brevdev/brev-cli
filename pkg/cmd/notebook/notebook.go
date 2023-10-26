// notebook/notebook.go
package notebook

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/portforward"
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

func NewCmdNotebook(store NotebookStore, t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "notebook",
		Short:   "Open a notebook on your Brev machine",
		Long:    notebookLong,
		Example: notebookExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			urlType := color.New(color.FgCyan, color.Bold).SprintFunc()
			warningType := color.New(color.FgBlack, color.Bold, color.BgCyan).SprintFunc()

			fmt.Print("\n")
			fmt.Println(warningType("  Please keep this terminal open 🤙  "))

			fmt.Println("\nClick here to go to your Jupyter notebook:\n\t 👉", urlType("http://localhost:8888"), "👈\n\n")

			// Port forward on 8888
			err := portforward.RunPortforward(store, args[0], "8888:8888")
			if err != nil {
				return err
			}

			// Print out a link for the user
			fmt.Println("Your notebook is accessible at: http://localhost:8888")

			return nil
		},
	}

	return cmd
}
