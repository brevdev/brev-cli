package reset

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	startLong    = "Reset your machine if it's acting up. This deletes the machine and gets you a fresh one."
	startExample = "brev reset <ws_name>"
)

type ResetStore interface {
	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
}

func NewCmdReset(t *terminal.Terminal, loginResetStore ResetStore, noLoginResetStore ResetStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "reset",
		DisableFlagsInUseLine: true,
		Short:                 "Reset a workspace if it's in a weird state.",
		Long:                  startLong,
		Example:               startExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginResetStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			err := resetWorkspace(args[0], t, loginResetStore)
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}
		},
	}

	return cmd
}

func resetWorkspace(workspaceName string, t *terminal.Terminal, resetStore ResetStore) error {
	org, err := resetStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return fmt.Errorf("no orgs exist")
	}

	workspaces, err := resetStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{Name: workspaceName})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if len(workspaces) == 0 {
		return fmt.Errorf("no workspace exist with name %s", workspaceName)
	} else if len(workspaces) > 1 {
		return fmt.Errorf("more than 1 workspace exist with name %s", workspaceName)
	}

	workspace := workspaces[0]

	startedWorkspace, err := resetStore.ResetWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("Workspace %s is resetting. \n Note: this can take a few seconds. Run 'brev ls' to check status", startedWorkspace.Name)

	return nil
}
