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

			user, _ := store.GetCurrentUser()
			hello.CanWeOnboard(t, user, store)

			currentOnboardingStatus, err := user.GetOnboardingData()
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}

			t.Vprintf("\nOnboarding Skipped: %v", prettyPrintBoolean(t, currentOnboardingStatus.CliOnboardingSkipped))
			t.Vprintf("\nCliOnboardingIntro: %v", prettyPrintBoolean(t, currentOnboardingStatus.CliOnboardingIntro))
			t.Vprintf("\nCliOnboardingLs: %v", prettyPrintBoolean(t, currentOnboardingStatus.CliOnboardingLs))
			t.Vprintf("\nCliOnboardingBrevOpen: %v", prettyPrintBoolean(t, currentOnboardingStatus.CliOnboardingBrevOpen))
			t.Vprintf("\nCliOnboardingBrevShell: %v", prettyPrintBoolean(t, currentOnboardingStatus.CliOnboardingBrevShell))
			t.Vprintf("\nCliOnboardingCompleted: %v", prettyPrintBoolean(t, currentOnboardingStatus.CliOnboardingCompleted))
			t.Vprintf("\n")
			return nil
		},
	}

	return cmd
}

func prettyPrintBoolean(t *terminal.Terminal, res bool) string {
	if res {
		return t.Green("%v", res)
	} else {
		return t.Red("%v", res)
	}
}
