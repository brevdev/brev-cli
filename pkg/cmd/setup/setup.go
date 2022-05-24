// Package setup WE SHOULD SETUP THE USER HERE
package setup

import (
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	setupLong    = "Setup your local machine to work best with Brev.dev"
	setupExample = "brev setup"
)

type SetupStore interface {
	GetCurrentUser() (*entity.User, error)
	CreateUser(idToken string) (*entity.User, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	CreateOrganization(req store.CreateOrganizationRequest) (*entity.Organization, error)
	GetServerSockFile() string
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
}

func NewCmdOpen(_ *terminal.Terminal, _ SetupStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "open",
		DisableFlagsInUseLine: true,
		Short:                 "[beta] open VSCode to ",
		Long:                  setupLong,
		Example:               setupExample,
		Args:                  cmderrors.TransformToBrevArgs(cobra.ExactArgs(1)),
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return cmd
}
