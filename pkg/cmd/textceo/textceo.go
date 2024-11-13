package textceo

import (
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
)

var (
	startLong    = "Send a text message to our CEO, Nader"
	startExample = "brev text-ceo -m 'Hi Nader, why is Microsoft a scorpion?'"
)

type TextCEOStore interface {
	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
}

// cmd.Flags().StringVarP(&message, "message", "m", "", "message to send Brev's CEO, Nader")
// cmd.Flags().StringVarP(&message, "phone", "p", "", "(Optional) leave a number for Nader to follow up")
