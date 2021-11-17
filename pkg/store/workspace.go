package store

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type GetWorkspacesOptions struct {
	UserID string
}

func (s AuthHTTPStore) GetWorkspaces(organizationID string, options *GetWorkspacesOptions) ([]brevapi.Workspace, error) {
	workspaces, err := s.getWorkspaces(organizationID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if options == nil || options.UserID == "" {
		return workspaces, nil
	}

	var myWorkspaces []brevapi.Workspace
	for _, w := range workspaces {
		if w.CreatedByUserID == options.UserID {
			myWorkspaces = append(myWorkspaces, w)
		}
	}

	return myWorkspaces, nil
}

var (
	orgIDParamName          = "organizationID"
	workspaceOrgPathPattern = "api/organizations/%s/workspaces"
	workspaceOrgPath        = fmt.Sprintf(workspaceOrgPathPattern, fmt.Sprintf("{%s}", orgIDParamName))
)

func (s AuthHTTPStore) getWorkspaces(organizationID string) ([]brevapi.Workspace, error) {
	var result []brevapi.Workspace
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(orgIDParamName, organizationID).
		SetResult(&result).
		Get(workspaceOrgPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return result, nil
}

var (
	workspaceIDParamName         = "workspaceID"
	workspaceMetadataPathPattern = "api/workspaces/%s/metadata"
	workspaceMetadataPath        = fmt.Sprintf(workspaceMetadataPathPattern, fmt.Sprintf("{%s}", workspaceIDParamName))
)

func (s AuthHTTPStore) GetWorkspaceMetaData(workspaceID string) (*brevapi.WorkspaceMetaData, error) {
	var result brevapi.WorkspaceMetaData
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetResult(&result).
		Get(workspaceMetadataPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}
