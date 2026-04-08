package util

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
)

// OrgResolver is the interface needed to resolve an org from a --org flag.
type OrgResolver interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
}

// ResolveOrgFromFlag resolves an organization from the --org flag value.
// If orgFlag is empty, returns the active org. If orgFlag is set, looks up the org by name.
func ResolveOrgFromFlag(s OrgResolver, orgFlag string) (*entity.Organization, error) {
	if orgFlag == "" {
		org, err := s.GetActiveOrganizationOrDefault()
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if org == nil {
			return nil, breverrors.NewValidationError("no orgs exist")
		}
		return org, nil
	}
	orgs, err := s.GetOrganizations(&store.GetOrganizationsOptions{Name: orgFlag})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("no org found with name %s", orgFlag))
	} else if len(orgs) > 1 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("more than one org found with name %s", orgFlag))
	}
	return &orgs[0], nil
}

type GetWorkspaceByNameOrIDErrStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error)
	GetCurrentUser() (*entity.User, error)
}

func GetUserWorkspaceByNameOrIDErr(storeQ GetWorkspaceByNameOrIDErrStore, workspaceNameOrID string) (*entity.Workspace, error) {
	org, err := storeQ.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return GetUserWorkspaceByNameOrIDErrInOrg(storeQ, workspaceNameOrID, org.ID)
}

// GetUserWorkspaceByNameOrIDErrInOrg looks up a workspace by name or ID in a specific org.
func GetUserWorkspaceByNameOrIDErrInOrg(storeQ GetWorkspaceByNameOrIDErrStore, workspaceNameOrID string, orgID string) (*entity.Workspace, error) {
	user, err := storeQ.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	workspaces, err := storeQ.GetWorkspaceByNameOrID(orgID, workspaceNameOrID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	workspaces = store.FilterForUserWorkspaces(workspaces, user.ID)
	if len(workspaces) == 0 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("instance with id/name %s not found", workspaceNameOrID))
	}
	return &workspaces[0], nil
}

func GetAnyWorkspaceByIDOrNameInActiveOrgErr(storeQ GetWorkspaceByNameOrIDErrStore, workspaceNameOrID string) (*entity.Workspace, error) {
	org, err := storeQ.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return GetAnyWorkspaceByIDOrNameInOrgErr(storeQ, workspaceNameOrID, org.ID)
}

// GetAnyWorkspaceByIDOrNameInOrgErr looks up any workspace (not just the user's) by name or ID in a specific org.
func GetAnyWorkspaceByIDOrNameInOrgErr(storeQ GetWorkspaceByNameOrIDErrStore, workspaceNameOrID string, orgID string) (*entity.Workspace, error) {
	workspaces, err := storeQ.GetWorkspaceByNameOrID(orgID, workspaceNameOrID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if len(workspaces) == 0 {
		return nil, breverrors.NewValidationError(fmt.Sprintf("instance with id/name %s not found", workspaceNameOrID))
	}
	if len(workspaces) > 1 {
		workspaces = store.FilterNonFailedWorkspaces(workspaces)
		if len(workspaces) == 0 {
			return nil, breverrors.NewValidationError(fmt.Sprintf("instance with id/name %s is a failed workspace", workspaceNameOrID))
		}
		if len(workspaces) > 1 {
			return nil, breverrors.NewValidationError(fmt.Sprintf("multiple instances found with id/name %s", workspaceNameOrID))
		}
	}
	return &workspaces[0], nil
}

type MakeWorkspaceWithMetaStore interface {
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
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
	if w.InstanceType != "" {
		return w.InstanceType
	}
	if w.WorkspaceClassID != "" {
		return GetClassIDString(w.WorkspaceClassID)
	}
	return ""
}
