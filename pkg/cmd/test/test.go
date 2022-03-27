package test

import (
	"time"

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
}

func NewCmdTest(t *terminal.Terminal, _ TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "scale",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		// Args:                  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			
			t.Vprintf("You currently have a %s\n", t.Yellow("2x8"))
			// t.Vprint("\n")

			upgradeType := terminal.PromptSelectInput(terminal.PromptSelectContent{
				Label:    "How much do you want to upgrade to: ",
				ErrorMsg: "error",
				Items:    []string{"2x4", "4x16", "8x32"},
			})


			bar := t.NewProgressBar("Upgrading to 8 CPUs and 32GB of RAM", nil)
			bar.AdvanceTo(5);
			time.Sleep(5);
			bar.AdvanceTo(100);

			t.Vprint(t.Green("\n\nEnjoy your %s 🤙", t.Green(upgradeType)))

			
			t.Vprint("\n");
			return nil
		},
	}

	return cmd
}
