// Package stop is for stopping Brev workspaces
package stop

import (
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	stopLong    = "Stop a Brev machine that's in a running state"
	stopExample = "brev stop <ws_name>... \nbrev stop --all"
)

type StopStore interface {
	completions.CompletionStore
	util.GetWorkspaceByNameOrIDErrStore
	StopWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentUser() (*entity.User, error)
	IsWorkspace() (bool, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
}

func NewCmdStop(t *terminal.Terminal, loginStopStore StopStore, noLoginStopStore StopStore) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "stop",
		DisableFlagsInUseLine: true,
		Short:                 "Stop a workspace if it's running",
		Long:                  stopLong,
		Example:               stopExample,
		// Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs()),
		ValidArgsFunction: completions.GetAllWorkspaceNameCompletionHandler(noLoginStopStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return stopAllWorkspaces(t, loginStopStore)
			} else {
				if len(args) == 0 {
					return breverrors.NewValidationError("please provide a workspace to stop")
				}
				for _, arg := range args {
					err := stopWorkspace(arg, t, loginStopStore)
					if err != nil {
						return breverrors.WrapAndTrace(err)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "stop all workspaces")

	return cmd
}

func stopAllWorkspaces(t *terminal.Terminal, stopStore StopStore) error {
	user, err := stopStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	org, err := stopStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaces, err := stopStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{UserID: user.ID})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprintf("\nTurning off all of your workspaces")
	for _, v := range workspaces {
		if v.Status == "RUNNING" {
			_, err = stopStore.StopWorkspace(v.ID)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			} else {
				t.Vprintf(t.Green("\n%s stopped ✓", v.Name))
			}
		}
	}
	return nil
}

func stopWorkspace(workspaceName string, t *terminal.Terminal, stopStore StopStore) error {
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(stopStore, workspaceName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = stopStore.StopWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf(t.Green("Workspace "+workspace.Name+" is stopping.") +
		"\nNote: this can take a few seconds. Run 'brev ls' to check status\n")

	return nil
}

func StopThisWorkspace(store StopStore, _ *terminal.Terminal) error {
	isWorkspace, err := store.IsWorkspace()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if isWorkspace {
		_ = 0
		// get current workspace
		// stopWorkspace("")
		// stop the workspace
	} else {
		return breverrors.NewValidationError("this is not a workspace -- please provide a workspace id")
	}
	return nil
}
