// Package environment provides shared operations for DevPlane environments.
package environment

import (
	"context"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// maxEnvironmentPageSize caps ListOrganizationComputeResources page size at 100.
const maxEnvironmentPageSize = 100

// Client exposes the DevPlane environment and organization APIs and common read operations.
type Client struct {
	devplanev1connect.EnvironmentServiceClient
	devplanev1connect.OrganizationServiceClient
}

// NewClient adds environment helpers to DevPlane API clients.
func NewClient(environmentClient devplanev1connect.EnvironmentServiceClient, organizationClient devplanev1connect.OrganizationServiceClient) Client {
	return Client{
		EnvironmentServiceClient:  environmentClient,
		OrganizationServiceClient: organizationClient,
	}
}

// EnvironmentStatus reads a single environment's authoritative lifecycle state.
func (c Client) EnvironmentStatus(ctx context.Context, environmentID string) (Status, error) {
	response, err := c.EnvironmentServiceClient.GetEnvironment(ctx, connect.NewRequest(&devplanev1.GetEnvironmentRequest{
		EnvironmentId:       environmentID,
		AttachedDataOptions: &devplanev1.GetEnvironmentAttachedDataOptions{Instance: true},
	}))
	if err != nil {
		return Status{}, breverrors.WrapAndTrace(err)
	}
	return resolveStatus(response.Msg.GetEnvironment()), nil
}

// EnvironmentStatusByOrg reads the authoritative lifecycle state of every environment in the
// organization, keyed by environment ID.
func (c Client) EnvironmentStatusByOrg(ctx context.Context, organizationID string) (map[string]Status, error) {
	statuses := make(map[string]Status)
	pageToken := ""
	for {
		response, err := c.OrganizationServiceClient.ListOrganizationComputeResources(ctx, connect.NewRequest(&devplanev1.ListOrganizationComputeResourcesRequest{
			OrganizationId: organizationID,
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
				statuses[env.GetEnvironmentId()] = resolveStatus(env)
			}
		}
		pageToken = response.Msg.GetNextPageToken()
		if pageToken == "" {
			return statuses, nil
		}
	}
}
