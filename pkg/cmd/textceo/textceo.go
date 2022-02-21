package textceo

import (
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	startLong    = "Send a text message to our CEO, Nader"
	startExample = "brev text-ceo -m 'Hi Nader, why is Microsoft a scorpion?'"
)

type TextCEOStore interface {
	var message string
	var phone string

	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
}

func NewCmdTextCEO(t *terminal.Terminal, _ TextCEOStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "Send a text message to our CEO, Nader",
		Long:                  startLong,
		Example:               startExample,
		Run: func(cmd *cobra.Command, args []string) {

			t.Vprint("\ntest cmd\n")

		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "message to send Brev's CEO, Nader")
	cmd.Flags().StringVarP(&message, "phone", "p", "", "(Optional) leave a number for Nader to follow up")

	return cmd
}
