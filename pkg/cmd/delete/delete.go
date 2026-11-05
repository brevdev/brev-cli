package delete

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/environment"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	stripmd "github.com/writeas/go-strip-markdown"
)

var (
	//go:embed doc.md
	deleteLong    string
	deleteExample = "brev delete <instance_name_or_id>...\necho instance-name | brev delete"
)

type DeleteStore interface {
	completions.CompletionStore
	DeleteWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error)
	GetEnvironment(ctx context.Context, environmentID string) (*devplanev1.Environment, error)
}

func NewCmdDelete(t *terminal.Terminal, loginDeleteStore DeleteStore, noLoginDeleteStore DeleteStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "delete",
		DisableFlagsInUseLine: true,
		Short:                 "Delete an instance",
		Long:                  stripmd.Strip(deleteLong),
		Example:               deleteExample,
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginDeleteStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			piped := util.IsStdoutPiped()
			names, err := util.GetInstanceNames(args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			var allError error
			var deletedNames []string
			for _, workspace := range names {
				err := deleteWorkspace(cmd.Context(), workspace, t, loginDeleteStore, piped)
				if err != nil {
					allError = multierror.Append(allError, err)
				} else {
					deletedNames = append(deletedNames, workspace)
				}
			}
			if allError != nil {
				return breverrors.WrapAndTrace(allError)
			}
			// Only output names for piping if all succeeded
			if piped {
				for _, name := range deletedNames {
					fmt.Println(name)
				}
			}
			return nil
		},
	}

	return cmd
}

func deleteWorkspace(ctx context.Context, workspaceNameOrID string, t *terminal.Terminal, deleteStore DeleteStore, piped bool) error {
	workspace, found, err := util.FindUserWorkspaceByNameOrID(deleteStore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !found && !auth.IsAPIKeyAuthStore(deleteStore) {
		workspace, found, err = getOrgWorkspace(deleteStore, workspaceNameOrID, piped)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	if !found {
		isSystemAdmin, err := auth.IsCurrentUserSystemAdmin(deleteStore)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if !isSystemAdmin {
			return breverrors.NewValidationError(fmt.Sprintf("instance with id/name %s not found", workspaceNameOrID))
		}
		if !piped {
			fmt.Fprintln(os.Stderr, "attempting to delete an instance you don't own as admin")
		}
	}

	var workspaceID string
	if workspace != nil {
		env, err := deleteStore.GetEnvironment(ctx, workspace.ID)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if phase := setupPhase(workspace, env); phase != "" {
			return breverrors.NewValidationError(fmt.Sprintf(
				"instance %q is still %s and can't be deleted yet. Run 'brev ls' and try again once it's running.",
				workspace.Name, phase))
		}
		if isDeleting(workspace, env) {
			return breverrors.NewValidationError(fmt.Sprintf("instance %q is already being deleted", workspace.Name))
		}
		workspaceID = workspace.ID
	} else {
		workspaceID = workspaceNameOrID
	}

	deletedWorkspace, err := deleteStore.DeleteWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if !piped {
		t.Vprintf("Deleting instance %s. This can take a few minutes. Run 'brev ls' to check status\n", deletedWorkspace.Name)
	}

	return nil
}

func setupPhase(workspace *entity.Workspace, env *devplanev1.Environment) string {
	if env.GetStatus() == devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_UNSPECIFIED && workspace.Status == entity.Deploying {
		return "deploying"
	}
	return environment.SetupPhase(env)
}

func isDeleting(workspace *entity.Workspace, env *devplanev1.Environment) bool {
	if env.GetStatus() == devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_UNSPECIFIED {
		return workspace.Status == entity.Deleting
	}
	return environment.Display(env) == entity.Deleting
}

func getOrgWorkspace(deleteStore DeleteStore, workspaceName string, piped bool) (*entity.Workspace, bool, error) {
	org, err := deleteStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, false, breverrors.WrapAndTrace(err)
	}
	workspaces, err := deleteStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{Name: workspaceName})
	if err != nil {
		return nil, false, breverrors.WrapAndTrace(err)
	}
	switch len(workspaces) {
	case 0:
		return nil, false, nil
	case 1:
		if !piped {
			fmt.Fprintf(os.Stderr, "%s belongs to another member of the org\n", workspaces[0].Name)
		}
		return &workspaces[0], true, nil
	default:
		return nil, true, breverrors.NewValidationError(fmt.Sprintf("multiple instances found with name %s; use the instance ID from 'brev ls --all'", workspaceName))
	}
}
