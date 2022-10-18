package delete

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	stripmd "github.com/writeas/go-strip-markdown"
)

var (
	//go:embed doc.md
	deleteLong    string
	deleteExample = "brev delete <ws_name>"
)

type DeleteStore interface {
	completions.CompletionStore
	DeleteWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error)
}

func NewCmdDelete(t *terminal.Terminal, loginDeleteStore DeleteStore, noLoginDeleteStore DeleteStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "delete",
		DisableFlagsInUseLine: true,
		Short:                 "Delete a Brev dev environment",
		Long:                  stripmd.Strip(deleteLong),
		Example:               deleteExample,
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginDeleteStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			var allError error
			for _, workspace := range args {
				err := deleteWorkspace(workspace, t, loginDeleteStore)
				if err != nil {
					allError = multierror.Append(allError, err)
				}
			}
			if allError != nil {
				return breverrors.WrapAndTrace(allError)
			}
			return nil
		},
	}

	return cmd
}

func deleteWorkspace(workspaceName string, t *terminal.Terminal, deleteStore DeleteStore) error {
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(deleteStore, workspaceName)
	if err != nil {
		err1 := handleAdminUser(err, deleteStore)
		if err1 != nil {
			return breverrors.WrapAndTrace(err1)
		}
	}

	var workspaceID string
	if workspace != nil {
		workspaceID = workspace.ID
	} else {
		workspaceID = workspaceName
	}

	deletedWorkspace, err := deleteStore.DeleteWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("Deleting dev environment %s. This can take a few minutes. Run 'brev ls' to check status\n", deletedWorkspace.Name)

	return nil
}

func handleAdminUser(err error, deleteStore DeleteStore) error {
	if strings.Contains(err.Error(), "not found") {
		user, err1 := deleteStore.GetCurrentUser()
		if err1 != nil {
			return breverrors.WrapAndTrace(err1)
		}
		if user.GlobalUserType != "Admin" {
			return breverrors.WrapAndTrace(err)
		}
		fmt.Println("attempting to delete a workspace you don't own as admin")
		return nil
	}

	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
