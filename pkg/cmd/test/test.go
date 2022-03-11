package test

import (
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
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

}

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
	GetCurrentWorkspaceID() string
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdTest(t *terminal.Terminal, ts TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		// Args:                  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			currOrg, err := ts.GetActiveOrganizationOrDefault()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			// Get Current User
			currentUser, err := ts.GetCurrentUser()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			wss, err := ts.GetWorkspaces(currOrg.ID, &store.GetWorkspacesOptions{UserID: currentUser.ID})
			
			for _, v := range wss {
				t.Vprintf("%s - %s - %s\n",v.Name, v.ID, v.GitRepo)
			}

			t.Vprint(t.Green("~~~ All Workspaces ~~~~"))
			getUnjoinedWorkspaces(ts, currentUser.ID, currOrg.ID, t)

			return nil

		},
	}

	return cmd
}

func getUnjoinedWorkspaces(ts TestStore, userID string, orgID string, t *terminal.Terminal) error {
	wss, err := ts.GetWorkspaces(orgID, nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)

	}

	listJoinedByGitURL := make(map[string][]entity.Workspace)

	var notMyWorkspaces []entity.Workspace
	for _, w := range wss {
		if w.CreatedByUserID == userID {
			l := listJoinedByGitURL[w.GitRepo]
			l = append(l, w)
			listJoinedByGitURL[w.GitRepo] = l
		} else {
			notMyWorkspaces = append(notMyWorkspaces, w)
		}
	}

	listByGitURL := make(map[string][]entity.Workspace)
	for _, w := range notMyWorkspaces {

		_, exist := listJoinedByGitURL[w.GitRepo]

		// unjoined workspaces only: check it's not a joined one
		if !exist {
			l := listByGitURL[w.GitRepo]
			l = append(l, w)
			listByGitURL[w.GitRepo] = l
		}

	}
	var unjoinedWorkspaces []entity.Workspace
	for gitURL := range listByGitURL {
		unjoinedWorkspaces = append(unjoinedWorkspaces, listByGitURL[gitURL][0])
	}
	
	for _, v := range unjoinedWorkspaces {
		t.Vprintf("%s - %s - %s\n", len(listByGitURL[v.GitRepo]), v.Name, v.GitRepo)
	}


	return nil
}

// cfg := autostartconf.LinuxSystemdConfigurer{
// 	AutoStartStore: store,
// 	ValueConfigFile: `
// [Install]
// WantedBy=multi-user.target

// [Unit]
// Description=Brev SSH Proxy Daemon
// After=systemd-user-sessions.service

// [Service]
// Type=simple
// ExecStart=brev run-tasks
// Restart=always
// `, DestConfigFile: "/etc/systemd/system/brev.service",
// }
// err := cfg.Install()
// if err != nil {
// 	return breverrors.WrapAndTrace(err)
// }
// return nil