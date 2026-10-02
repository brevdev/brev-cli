package environment

import (
	"context"
	"testing"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/entity"
)

type fakeEnvironmentClient struct {
	devplanev1connect.EnvironmentServiceClient
	environments map[string]*devplanev1.Environment
	gotID        string
}

func (f *fakeEnvironmentClient) GetEnvironment(
	_ context.Context,
	request *connect.Request[devplanev1.GetEnvironmentRequest],
) (*connect.Response[devplanev1.GetEnvironmentResponse], error) {
	f.gotID = request.Msg.GetEnvironmentId()
	return connect.NewResponse(&devplanev1.GetEnvironmentResponse{
		Environment: f.environments[f.gotID],
	}), nil
}

type fakeOrganizationClient struct {
	devplanev1connect.OrganizationServiceClient
	pages    []*devplanev1.ListOrganizationComputeResourcesResponse
	calls    int
	gotOrgID string
}

func (f *fakeOrganizationClient) ListOrganizationComputeResources(
	_ context.Context,
	request *connect.Request[devplanev1.ListOrganizationComputeResourcesRequest],
) (*connect.Response[devplanev1.ListOrganizationComputeResourcesResponse], error) {
	f.gotOrgID = request.Msg.GetOrganizationId()
	page := f.pages[f.calls]
	f.calls++
	return connect.NewResponse(page), nil
}

func item(env *devplanev1.Environment) *devplanev1.ComputeResourceListItem {
	listItem := &devplanev1.ComputeResourceListItem{}
	listItem.SetEnvironment(env)
	return listItem
}

func TestEnvironmentStatusReadsSingleEnvironment(t *testing.T) {
	envClient := &fakeEnvironmentClient{environments: map[string]*devplanev1.Environment{
		"env-1": {
			EnvironmentId: "env-1",
			Name:          "box",
			Status:        devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED,
			Instance: &devplanev1.Instance{
				ProvisionStatus: &devplanev1.CreateAttemptStatus{Message: "out of quota"},
			},
		},
	}}
	client := NewClient(envClient, &fakeOrganizationClient{})

	status, err := client.EnvironmentStatus(context.Background(), "env-1")

	require.NoError(t, err)
	assert.Equal(t, "env-1", envClient.gotID)
	assert.True(t, status.Failed)
	assert.Equal(t, "box", status.Name)
	assert.Equal(t, "out of quota", status.Message)
}

func TestEnvironmentStatusByOrgPaginatesAndMaps(t *testing.T) {
	orgClient := &fakeOrganizationClient{pages: []*devplanev1.ListOrganizationComputeResourcesResponse{
		{
			Items: []*devplanev1.ComputeResourceListItem{
				item(&devplanev1.Environment{
					EnvironmentId: "env-1",
					Name:          "a",
					Status:        devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING,
				}),
			},
			NextPageToken: "next",
		},
		{
			Items: []*devplanev1.ComputeResourceListItem{
				item(&devplanev1.Environment{
					EnvironmentId: "env-2",
					Name:          "b",
					Status:        devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPED,
				}),
			},
		},
	}}
	client := NewClient(&fakeEnvironmentClient{}, orgClient)

	statuses, err := client.EnvironmentStatusByOrg(context.Background(), "org-1")

	require.NoError(t, err)
	assert.Equal(t, "org-1", orgClient.gotOrgID)
	assert.Equal(t, 2, orgClient.calls, "expected both pages to be fetched")
	require.Len(t, statuses, 2)
	assert.True(t, statuses["env-1"].Ready)
	assert.Equal(t, entity.Running, statuses["env-1"].Display)
	assert.Equal(t, entity.Stopped, statuses["env-2"].Display)
}
