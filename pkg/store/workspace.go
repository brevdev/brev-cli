package store

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type GetWorkspacesOptions struct {
	UserID string
}

func (s AuthHTTPStore) GetWorkspaces(organizationID string, options GetWorkspacesOptions) ([]brevapi.Workspace, error) {
	workspaces, err := s.authHTTPClient.toDeprecateClient.GetWorkspaces(organizationID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	currentUser, err := s.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if options.UserID != "" {
		var myWorkspaces []brevapi.Workspace
		for _, w := range workspaces {
			if w.CreatedByUserID == currentUser.ID {
				myWorkspaces = append(myWorkspaces, w)
			}
		}
		workspaces = myWorkspaces
	}
	return workspaces, nil
}

func (s AuthHTTPStore) GetWorkspaceMetaData(workspaceID string) (*brevapi.WorkspaceMetaData, error) {
	meta, err := s.authHTTPClient.toDeprecateClient.GetWorkspaceMetaData(workspaceID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return meta, nil
}
