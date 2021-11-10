// Package link is for the ssh command
package delete

import (
	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	deleteLong    = "Delete a Brev workspace that you no longer need. If you have a .brev setup script, you can get a new one without setting up."
	deleteExample = "brev delete <ws_name>"
)

// func getWorkspaceNames() []string {
// 	activeOrg, err := brev_api.GetActiveOrgContext()
// 	if err != nil {
// 		return nil
// 	}

// 	client, err := brev_api.NewCommandClient()
// 	if err != nil {
// 		return nil
// 	}
// 	wss, err := client.GetMyWorkspaces(activeOrg.ID)
// 	if err != nil {
// 		return nil
// 	}

// 	var wsNames []string
// 	for _, w := range wss {
// 		wsNames = append(wsNames, w.Name)
// 	}

// 	return wsNames
// }

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
		ValidArgs:             getCachedWorkspaceNames(),
		Run: func(cmd *cobra.Command, args []string) {

			if len(args) > 0 {
				t.Vprint(args[0])
			}

		},
	}

	return cmd
}
