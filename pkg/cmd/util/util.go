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
		return nil, breverrors.NewValidationError(fmt.Sprintf("dev environment with id/name %s not found", workspaceNameOrID))
	}
	if len(workspaces) > 1 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("multiple dev environments found with id/name %s", workspaceNameOrID))
	}
	return &workspaces[0], nil
}

func GetAnyWorkspaceByIDOrNameInActiveOrgErr(storeQ GetWorkspaceByNameOrIDErrStore, workspaceNameOrID string) (*entity.Workspace, error) {
	org, err := storeQ.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	workspaces, err := storeQ.GetWorkspaceByNameOrID(org.ID, workspaceNameOrID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if len(workspaces) == 0 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("dev environment with id/name %s not found", workspaceNameOrID))
	}
	if len(workspaces) > 1 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("multiple dev environments found with id/name %s", workspaceNameOrID))
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

func GetClassIDString(classID string) string {
	// switch statement on class ID
	switch classID {
	case "2x2":
		return "2 cpu | 2 gb ram"
	case "2x4":
		return "2 cpu | 4 gb ram"
	case "2x8":
		return "2 cpu | 8 gb ram"
	case "4x16":
		return "4 cpu | 16 gb ram"
	case "8x32":
		return "8 cpu | 32 gb ram"
	case "16x32":
		return "16 cpu | 32 gb ram"
	default:
		return classID

	}
}

func GetInstanceString(w entity.Workspace) string {
	var instanceString string
	if w.WorkspaceClassID != "" {
		instanceString = GetClassIDString(w.WorkspaceClassID)
	} else {
		instanceString = w.InstanceType + " (gpu)"
	}
	return instanceString
}
