package store

import "github.com/brevdev/brev-cli/pkg/brevapi"

type GetWorkspacesOptions struct {
	OrganizationID string
}

func (s AuthHTTPStore) GetWorkspaces(_ GetWorkspacesOptions) ([]brevapi.Workspace, error) {
	// func (s AuthHTTPStore) GetWorkspaces(options GetWorkspacesOptions) ([]brevapi.Workspace, error) {
	return nil, nil
}
