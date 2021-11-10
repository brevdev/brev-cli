// Package link is for the ssh command
package delete

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	deleteLong    = "Delete a Brev workspace that you no longer need. If you have a .brev setup script, you can get a new one without setting up."
	deleteExample = "brev delete <ws_name>"
)

func NewCmdDelete(t *terminal.Terminal) *cobra.Command {
	// link [resource id] -p 2222
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "delete",
		DisableFlagsInUseLine: true,
		Short:                 "Delete a Brev workspace",
		Long:                  deleteLong,
		Example:               deleteExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             brev_api.GetCachedWorkspaceNames(),
		Run: func(cmd *cobra.Command, args []string) {

			err := deleteWorkspaceByID(args[0], t)
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}

		},
	}

	return cmd
}

func deleteWorkspaceByID(name string, t *terminal.Terminal) error {
	client, err := brev_api.NewCommandClient()
	if err != nil {
		return err
	}

	workspace, err := brev_api.GetWorkspaceFromName(name)
	if err != nil {
		return err
	}

	deletedWorkspace, err := client.DeleteWorkspace(workspace.ID)
	if err != nil {
		fmt.Println(err)
		return err
	}

	t.Vprintf("Deleting workspace %s. \n Note: this can take a few minutes. Run 'brev ls' to check status", deletedWorkspace.Name)

	return nil
}
