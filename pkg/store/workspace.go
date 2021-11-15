package store

import "github.com/brevdev/brev-cli/pkg/brev_api"

type GetWorkspacesOptions struct {
	OrganizationID string
}

func (s AuthHTTPStore) GetWorkspaces(options GetWorkspacesOptions) ([]brev_api.Workspace, error) {
	return nil, nil
}
