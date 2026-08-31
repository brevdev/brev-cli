package managedsecret

import (
	"context"
	"testing"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeManagedSecretClient struct {
	devplanev1connect.ManagedSecretServiceClient
	secretID string
	versions []*devplanev1.ManagedSecretVersion
}

func TestVersionIDResolvesCanonicalVersion(t *testing.T) {
	service := &fakeManagedSecretClient{versions: []*devplanev1.ManagedSecretVersion{
		{VersionId: "msecv-2", VersionNumber: 2},
		{VersionId: "msecv-1", VersionNumber: 1},
	}}
	client := NewClient(service)

	versionID, err := client.GetVersionIDForVersionNumber(t.Context(), "msec-1", 1)

	require.NoError(t, err)
	assert.Equal(t, "msec-1", service.secretID)
	assert.Equal(t, "msecv-1", versionID)
}

func (f *fakeManagedSecretClient) ListSecretVersions(
	_ context.Context,
	request *connect.Request[devplanev1.ManagedSecretServiceListSecretVersionsRequest],
) (*connect.Response[devplanev1.ManagedSecretServiceListSecretVersionsResponse], error) {
	f.secretID = request.Msg.GetSecretId()
	return connect.NewResponse(&devplanev1.ManagedSecretServiceListSecretVersionsResponse{Items: f.versions}), nil
}
