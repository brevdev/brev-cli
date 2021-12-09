package delete

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	deleteLong    = "Delete a Brev workspace that you no longer need. If you have a .brev setup script, you can get a new one without setting up."
	deleteExample = "brev delete <ws_name>"
)

type DeleteStore interface {
	completions.CompletionStore
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	DeleteWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentUser() (*entity.User, error)
}

func NewCmdDelete(t *terminal.Terminal, loginDeleteStore DeleteStore, noLoginDeleteStore DeleteStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "delete",
		DisableFlagsInUseLine: true,
		Short:                 "Delete a Brev workspace",
		Long:                  deleteLong,
		Example:               deleteExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginDeleteStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			err := deleteWorkspace(args[0], t, loginDeleteStore)
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}
		},
	}

	return cmd
}

func deleteWorkspace(workspaceName string, t *terminal.Terminal, deleteStore DeleteStore) error {
	var workspaces []entity.Workspace
	org, err := deleteStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return fmt.Errorf("no orgs exist")
	} else {
		currentUser, err2 := deleteStore.GetCurrentUser()
		if err2 != nil {
			return breverrors.WrapAndTrace(err2)
		}
		workspaces, err = deleteStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{Name: workspaceName, UserID: currentUser.ID})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	var workspace entity.Workspace
	if len(workspaces) == 0 { //nolint:gocritic // gocritic recommends using a switch
		return fmt.Errorf("no workspaces found with name %s", workspaceName)
	} else if len(workspaces) > 1 {
		return fmt.Errorf("multiple workspaces found with name %s", workspaceName)
	} else {
		workspace = workspaces[0]
	}

	deletedWorkspace, err := deleteStore.DeleteWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("Deleting workspace %s. \n Note: this can take a few minutes. Run 'brev ls' to check status\n", deletedWorkspace.Name)

	return nil
}
