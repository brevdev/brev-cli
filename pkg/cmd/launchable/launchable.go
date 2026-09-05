package launchable

import (
	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

type LaunchableCmdStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetAllInstanceTypesWithWorkspaceGroups(orgID string) (*gpusearch.AllInstanceTypesResponse, error)
	CreateEnvironmentLaunchable(organizationID string, req store.CreateEnvironmentLaunchableRequest) (*store.CreateLaunchableResponse, error)
	ValidateDockerCompose(req store.ValidateDockerComposeRequest) (*store.ValidateDockerComposeResponse, error)
}

func NewCmdLaunchable(t *terminal.Terminal, store LaunchableCmdStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"organization": ""},
		Use:         "launchable",
		Short:       "Launchable commands",
		Long:        "Create and manage launchables.",
	}

	cmd.AddCommand(NewCmdLaunchableCreate(t, store))

	return cmd
}
