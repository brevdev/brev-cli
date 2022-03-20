package test

import (
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	startLong    = "[internal] test"
	startExample = "[internal] test"
)

type TestStore interface {
	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	CopyBin(targetBin string) error
	GetSetupScriptContentsByURL(url string) (string, error)
	UpdateLocalBashScripts(wss []entity.Workspace, userID string) error
}

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
	GetCurrentWorkspaceID() string
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdTest(_ *terminal.Terminal, store TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		// Args:                  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, _ := store.GetActiveOrganizationOrDefault()
			// if err != nil {
			// 	breverrors.WrapAndTrace(err)
			// }
			// if org == nil {
			// 	return nil, fmt.Errorf("no orgs exist")
			// }
			currentUser, _ := store.GetCurrentUser()
			// if err != nil {
			// 	return nil, breverrors.WrapAndTrace(err)
			// }

			// For some reason this doesn't work
			// wss, _ := store.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{UserID: currentUser.ID})
			wss, _ := store.GetWorkspaces(org.ID, nil)

			store.UpdateLocalBashScripts(wss, currentUser.ID)

			return nil
		},
	}

	return cmd
}
