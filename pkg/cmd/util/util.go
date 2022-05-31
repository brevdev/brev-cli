package util

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type GetWorkspaceByNameOrIDErrStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error)
}

func GetWorkspaceByNameOrIDErr(store GetWorkspaceByNameOrIDErrStore, workspaceNameOrID string) (*entity.Workspace, error) {
	org, err := store.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	workspaces, err := store.GetWorkspaceByNameOrID(org.ID, workspaceNameOrID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if len(workspaces) == 0 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("workspace with id/name %s not found", workspaceNameOrID))
	}
	if len(workspaces) > 1 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("multiple workspaces found with id/name %s", workspaceNameOrID))
	}
	return &workspaces[0], nil
}
