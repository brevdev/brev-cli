package workspacegroups

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type WorkspaceGroupsStore interface {
	GetWorkspaceGroups(organizationID string) ([]entity.WorkspaceGroup, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
}

func NewCmdWorkspaceGroups(t *terminal.Terminal, store WorkspaceGroupsStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "workspacegroups",
		DisableFlagsInUseLine: true,
		Short:                 "TODO",
		Long:                  "TODO",
		Example:               "TODO",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunWorkspaceGroups(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func RunWorkspaceGroups(t *terminal.Terminal, args []string, store WorkspaceGroupsStore) error {
	org, err := store.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	wsgs, err := store.GetWorkspaceGroups(org.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println(wsgs)
	return nil
}
