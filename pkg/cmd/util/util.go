package util

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
)

type GetWorkspaceByNameOrIDErrStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error)
	GetCurrentUser() (*entity.User, error)
}

func GetUserWorkspaceByNameOrIDErr(storeQ GetWorkspaceByNameOrIDErrStore, workspaceNameOrID string) (*entity.Workspace, error) {
	user, err := storeQ.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	org, err := storeQ.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	workspaces, err := storeQ.GetWorkspaceByNameOrID(org.ID, workspaceNameOrID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	workspaces = store.FilterForUserWorkspaces(workspaces, user.ID)

	if len(workspaces) == 0 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("workspace with id/name %s not found", workspaceNameOrID))
	}
	if len(workspaces) > 1 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("multiple workspaces found with id/name %s", workspaceNameOrID))
	}
	return &workspaces[0], nil
}

func GetWorkspaceByNameOrIDErr(storeQ GetWorkspaceByNameOrIDErrStore, workspaceNameOrID string) (*entity.Workspace, error) {
	org, err := storeQ.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	workspaces, err := storeQ.GetWorkspaceByNameOrID(org.ID, workspaceNameOrID)
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

type MakeWorkspaceWithMetaStore interface {
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
}

func MakeWorkspaceWithMeta(store MakeWorkspaceWithMetaStore, workspace *entity.Workspace) (entity.WorkspaceWithMeta, error) {
	workspaceMetaData, err := store.GetWorkspaceMetaData(workspace.ID)
	if err != nil {
		return entity.WorkspaceWithMeta{}, breverrors.WrapAndTrace(err)
	}

	return entity.WorkspaceWithMeta{
		WorkspaceMetaData: *workspaceMetaData,
		Workspace:         *workspace,
	}, nil
}
