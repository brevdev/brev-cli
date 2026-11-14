package store

import (
	"context"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const maxEnvironmentPageSize = 100

// GetEnvironment reads a single environment with its instance attached.
// Callers derive display state with environment.Display/FailureMessage.
func (s *AuthHTTPStore) GetEnvironment(ctx context.Context, environmentID string) (*devplanev1.Environment, error) {
	env, err := s.DevPlane().Environments.GetEnvironment(ctx, connect.NewRequest(&devplanev1.GetEnvironmentRequest{
		EnvironmentId:       environmentID,
		AttachedDataOptions: &devplanev1.GetEnvironmentAttachedDataOptions{Instance: true},
	}))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return env.Msg.GetEnvironment(), nil
}

// GetEnvironmentsByOrg reads every environment in the organization.
// Callers derive display state with environment.Display/FailureMessage.
func (s *AuthHTTPStore) GetEnvironmentsByOrg(ctx context.Context, orgID string) ([]*devplanev1.Environment, error) {
	var environments []*devplanev1.Environment
	pageToken := ""
	for {
		response, err := s.DevPlane().Organizations.ListOrganizationComputeResources(ctx, connect.NewRequest(&devplanev1.ListOrganizationComputeResourcesRequest{
			OrganizationId: orgID,
			PageParams:     &devplanev1.PageParams{PageSize: maxEnvironmentPageSize, PageToken: pageToken},
			Options: &devplanev1.ListOrganizationComputeResourcesOptions{
				Type:               devplanev1.ComputeResourceType_COMPUTE_RESOURCE_TYPE_ENVIRONMENT,
				EnvironmentOptions: &devplanev1.ListEnvironmentAttachedDataOptions{Instance: true},
			},
		}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		for _, item := range response.Msg.GetItems() {
			if env := item.GetEnvironment(); env != nil {
				environments = append(environments, env)
			}
		}
		pageToken = response.Msg.GetNextPageToken()
		if pageToken == "" {
			return environments, nil
		}
	}
}
