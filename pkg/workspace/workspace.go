package workspace

import (
	"context"
	swagger "github.com/brevdev/brev-cli/brevclient"
)

// Get gets a workspace from brev's api, provided its name or ID
func Get(identifier string) (*swagger.Workspace, error) {
	client := swagger.NewAPIClient(swagger.NewConfiguration())
	ctx := context.Background()
	workspace, resp, err := client.WorkspacesApi.ApiWorkspacesIdGet(ctx, identifier)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		// TODO implement ApiWorkspacesNameGet
		return nil, err
	}


	return &workspace, nil

}