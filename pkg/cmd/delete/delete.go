// Package link is for the ssh command
package delete

import (
	"errors"
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	deleteLong    = "Delete a Brev workspace that you no longer need. If you have a .brev setup script, you can get a new one without setting up."
	deleteExample = "brev delete <ws_name>"
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

	workspace, err := getWorkspaceFromName(name)
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

func getWorkspaceFromName(name string) (*brev_api.Workspace, error) {

	client, err := brev_api.NewCommandClient()
	if err != nil {
		return nil, err
	}
	activeOrg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		return nil, err
	}
	
	if err != nil {
		// Get all orgs
		orgs, err2 := client.GetOrgs()
		if err2 != nil {
			return nil, err2
		}
		for _, o := range orgs {
			wss, err3 := client.GetWorkspaces(o.ID)
			if err3 != nil {
				return nil, err3
			}
			for _, w := range wss {
				if w.Name == name {
					return &w, nil
				}
			}
		}
	}
	// If active org, get all ActiveOrg workspaces
	wss, err := client.GetMyWorkspaces(activeOrg.ID)
	if err != nil {
		return nil, err
	}
	for _, v := range wss {
		if v.Name == name {
			return &v, nil
		}
	}

	return nil, errors.New("no workspace with that name")
}
