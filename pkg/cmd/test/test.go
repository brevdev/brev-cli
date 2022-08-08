package test

import (
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/server"
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
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
	server.RPCServerTaskStore
}

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdTest(t *terminal.Terminal, store TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		// Args:                  cmderrors.TransformToValidationError(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			// fmt.Printf("NAME   ID     URL      SOMETHING ELSE")
			// hello.TypeItToMe("\n\n\n")
			// hello.TypeItToMe("👆 this is the name of your environment (which you can use to open the environment)")
			// time.Sleep(1 * time.Second)
			// fmt.Printf("\332K\r")
			// fmt.Println("                                                                                     ")
			// hello.TypeItToMe("              👆 you can expose your localhost to this public URL")
			// time.Sleep(1 * time.Second)
			// fmt.Printf("\332K\r")
			// fmt.Printf("bye world")
			// fmt.Printf("bye world")

			allWorkspaces, err := store.GetWorkspaces("ejmrvoj8m", nil)
			if err != nil {
				return err
			}
			var workspaces []entity.Workspace
			for _, v := range allWorkspaces {
				if v.CreatedByUserID == "62ga2fflb" {
					workspaces = append(workspaces, v)
				}
			}

			hello.Step1(t, workspaces)

			return nil
		},
	}

	return cmd
}
