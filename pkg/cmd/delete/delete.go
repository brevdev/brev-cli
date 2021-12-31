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
	GetWorkspace(id string) (*entity.Workspace, error)
	DeleteWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
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

	workspace, err := getWorkspaceFromNameOrID(workspaceName, deleteStore)
	if err != nil {
		t.Vprint(err.Error())
	}

	deletedWorkspace, err := deleteStore.DeleteWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("Deleting workspace %s. This can take a few minutes. Run 'brev ls' to check status\n", deletedWorkspace.Name)

	return nil
}

func getWorkspaceFromNameOrID(nameOrID string, sstore DeleteStore) (*entity.WorkspaceWithMeta, error) {
	// Get Active Org
	org, err := sstore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, fmt.Errorf("no orgs exist")
	}

	// Get Current User
	currentUser, err := sstore.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// Get Workspaces for User
	var workspace *entity.Workspace // this will be the returned workspace
	workspaces, err := sstore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{Name: nameOrID, UserID: currentUser.ID})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if len(workspaces) == 0 {
		// In this case, check workspace by ID
		wsbyid, err := sstore.GetWorkspace(nameOrID) // Note: workspaceName is ID in this case
		if err != nil {
			return nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
		}
		if wsbyid != nil {
			workspace = wsbyid
		} else {
			// Can this case happen?
			return nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
		}
	} else if len(workspaces) > 1 {
		return nil, fmt.Errorf("multiple workspaces found with name %s\n\nTry running the command by id instead of name:\n\tbrev command <id>", nameOrID)
	} else {
		workspace = &workspaces[0]
	}

	if workspace == nil {
		return nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
	}

	// Get WorkspaceMetaData
	workspaceMetaData, err := sstore.GetWorkspaceMetaData(workspace.ID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &entity.WorkspaceWithMeta{WorkspaceMetaData: *workspaceMetaData, Workspace: *workspace}, nil

}