package ports

import (
	"bytes"
	"context"
	"testing"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/entity"
)

type fakeUpdateEnvironmentService struct {
	devplanev1connect.UnimplementedEnvironmentServiceHandler
	t             *testing.T
	expectedEnvID string
	ports         []*devplanev1.Port
	responsePort  *devplanev1.Port
	targetReq     *devplanev1.EnvironmentServiceSetPortTargetRequest
	sourcesReq    *devplanev1.EnvironmentServiceSetPortAllowedSourcesRequest
	protocolReq   *devplanev1.EnvironmentServiceSetHTTPPortProtocolRequest
	accessReq     *devplanev1.EnvironmentServiceSetHTTPPortAccessRequest
}

func (s *fakeUpdateEnvironmentService) GetNetworkInfo(
	_ context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceGetNetworkInfoRequest],
) (*connect.Response[devplanev1.EnvironmentServiceGetNetworkInfoResponse], error) {
	s.t.Helper()
	assert.Equal(s.t, s.expectedEnvID, req.Msg.GetEnvironmentId())
	return connect.NewResponse(&devplanev1.EnvironmentServiceGetNetworkInfoResponse{
		NetworkInfo: &devplanev1.EnvironmentNetworkInfo{
			Status: devplanev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_CONNECTED,
			Ports:  s.ports,
		},
	}), nil
}

func (s *fakeUpdateEnvironmentService) SetPortTarget(
	_ context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceSetPortTargetRequest],
) (*connect.Response[devplanev1.EnvironmentServiceSetPortTargetResponse], error) {
	s.targetReq = req.Msg
	return connect.NewResponse(&devplanev1.EnvironmentServiceSetPortTargetResponse{Port: s.responsePort}), nil
}

func (s *fakeUpdateEnvironmentService) SetPortAllowedSources(
	_ context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceSetPortAllowedSourcesRequest],
) (*connect.Response[devplanev1.EnvironmentServiceSetPortAllowedSourcesResponse], error) {
	s.sourcesReq = req.Msg
	return connect.NewResponse(&devplanev1.EnvironmentServiceSetPortAllowedSourcesResponse{Port: s.responsePort}), nil
}

func (s *fakeUpdateEnvironmentService) SetHTTPPortProtocol(
	_ context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceSetHTTPPortProtocolRequest],
) (*connect.Response[devplanev1.EnvironmentServiceSetHTTPPortProtocolResponse], error) {
	s.protocolReq = req.Msg
	return connect.NewResponse(&devplanev1.EnvironmentServiceSetHTTPPortProtocolResponse{Port: s.responsePort}), nil
}

func (s *fakeUpdateEnvironmentService) SetHTTPPortAccess(
	_ context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceSetHTTPPortAccessRequest],
) (*connect.Response[devplanev1.EnvironmentServiceSetHTTPPortAccessResponse], error) {
	s.accessReq = req.Msg
	return connect.NewResponse(&devplanev1.EnvironmentServiceSetHTTPPortAccessResponse{Port: s.responsePort}), nil
}

type fakeUpdateNodeService struct {
	devplanev1connect.UnimplementedExternalNodeServiceHandler
	node         *devplanev1.ExternalNode
	responsePort *devplanev1.Port
	protocolReq  *devplanev1.SetHTTPPortProtocolRequest
	accessReq    *devplanev1.SetHTTPPortAccessRequest
}

func (s *fakeUpdateNodeService) ListNodes(
	_ context.Context,
	_ *connect.Request[devplanev1.ListNodesRequest],
) (*connect.Response[devplanev1.ListNodesResponse], error) {
	return connect.NewResponse(&devplanev1.ListNodesResponse{
		Items: []*devplanev1.ExternalNode{s.node},
	}), nil
}

func (s *fakeUpdateNodeService) SetHTTPPortProtocol(
	_ context.Context,
	req *connect.Request[devplanev1.SetHTTPPortProtocolRequest],
) (*connect.Response[devplanev1.SetHTTPPortProtocolResponse], error) {
	s.protocolReq = req.Msg
	return connect.NewResponse(&devplanev1.SetHTTPPortProtocolResponse{Port: s.responsePort}), nil
}

func (s *fakeUpdateNodeService) SetHTTPPortAccess(
	_ context.Context,
	req *connect.Request[devplanev1.SetHTTPPortAccessRequest],
) (*connect.Response[devplanev1.SetHTTPPortAccessResponse], error) {
	s.accessReq = req.Msg
	return connect.NewResponse(&devplanev1.SetHTTPPortAccessResponse{Port: s.responsePort}), nil
}

func newUpdateEnvironmentStore() *fakeStore {
	return &fakeStore{
		workspaces: []entity.Workspace{{ID: "env123", Name: "my-instance", CreatedByUserID: "user1"}},
		user:       &entity.User{ID: "user1", Email: "me@example.com"},
		org:        &entity.Organization{ID: "org1"},
	}
}

func testHTTPPort(destinationPort int32) *devplanev1.Port {
	hostname := "demo.apps.run.brev.nvidia.com"
	return &devplanev1.Port{
		PortId:           "http-one",
		HttpProtocol:     devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTP,
		PortNumber:       443,
		ServerPort:       destinationPort,
		Hostname:         &hostname,
		AuthorizedEmails: []string{"me@example.com"},
	}
}

func TestUpdateEnvironmentDestinationAndAllowedSources(t *testing.T) {
	updated := testTCPPort("nport-one", 41001)
	updated.ServerPort = 9090
	updated.AllowedSources = []string{"203.0.113.10/32", "198.51.100.0/24"}
	service := &fakeUpdateEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		ports:         []*devplanev1.Port{testTCPPort("nport-one", 41001)},
		responsePort:  updated,
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)
	cmd := NewCmdUpdatePort(newUpdateEnvironmentStore())
	cmd.SetArgs([]string{
		"nport-one", "--destination-port", "9090",
		"--allow", "203.0.113.10/32", "--allow", "198.51.100.0/24", "--json",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	require.NoError(t, err)
	require.NotNil(t, service.targetReq)
	assert.Equal(t, "nport-one", service.targetReq.GetPortId())
	assert.Equal(t, int32(9090), service.targetReq.GetPortNumber())
	require.NotNil(t, service.sourcesReq)
	assert.Equal(t, []string{"203.0.113.10/32", "198.51.100.0/24"}, service.sourcesReq.GetAllowedSources().GetCidrBlocks())
	assert.JSONEq(t, `{
		"port_id":"nport-one",
		"endpoint":"global.prd.ga.run.brev.nvidia.com:41001",
		"public_port":41001,
		"destination_port":9090,
		"protocol":"TCP",
		"allowed_sources":["203.0.113.10/32","198.51.100.0/24"],
		"authorized_emails":[],
		"allow_public_unauthenticated":false,
		"type":"user"
	}`, out.String())
}

func TestUpdateExternalNodeHTTPProtocolAndPublicAccess(t *testing.T) {
	public := true
	updated := testHTTPPort(8443)
	updated.HttpProtocol = devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTPS
	updated.AuthorizedEmails = nil
	updated.AllowPublicUnauthenticated = &public
	service := &fakeUpdateNodeService{
		node: &devplanev1.ExternalNode{
			ExternalNodeId: "unode123",
			Name:           "my-node",
			Ports:          []*devplanev1.Port{testHTTPPort(8443)},
		},
		responsePort: updated,
	}
	_, handler := devplanev1connect.NewExternalNodeServiceHandler(service)
	newTestServer(t, handler)
	store := &fakeStore{
		user: &entity.User{ID: "user1", Email: "me@example.com"},
		org:  &entity.Organization{ID: "org1"},
	}
	cmd := NewCmdUpdatePort(store)
	cmd.SetArgs([]string{"http-one", "--protocol", "https", "--public", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	require.NoError(t, err)
	require.NotNil(t, service.protocolReq)
	assert.Equal(t, devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTPS, service.protocolReq.GetHttpProtocol())
	require.NotNil(t, service.accessReq)
	assert.Empty(t, service.accessReq.GetAuthorizedEmails())
	assert.True(t, service.accessReq.GetAllowPublicUnauthenticated())
	assert.Contains(t, out.String(), `"protocol": "HTTPS"`)
	assert.Contains(t, out.String(), `"allow_public_unauthenticated": true`)
}

func TestUpdateHTTPAuthorizedEmails(t *testing.T) {
	updated := testHTTPPort(8080)
	updated.AuthorizedEmails = []string{"one@example.com", "two@example.com"}
	service := &fakeUpdateEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		ports:         []*devplanev1.Port{testHTTPPort(8080)},
		responsePort:  updated,
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)
	cmd := NewCmdUpdatePort(newUpdateEnvironmentStore())
	cmd.SetArgs([]string{
		"http-one",
		"--authorize", "one@example.com", "--authorize", "two@example.com",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	require.NoError(t, err)
	require.NotNil(t, service.accessReq)
	assert.Equal(t, []string{"one@example.com", "two@example.com"}, service.accessReq.GetAuthorizedEmails().GetEmails())
	assert.False(t, service.accessReq.GetAllowPublicUnauthenticated())
	assert.Contains(t, out.String(), "Updated port http-one.")
}

func TestUpdateRejectsHTTPFlagsForRawPort(t *testing.T) {
	service := &fakeUpdateEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		ports:         []*devplanev1.Port{testTCPPort("nport-one", 41001)},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)
	cmd := NewCmdUpdatePort(newUpdateEnvironmentStore())
	cmd.SetArgs([]string{"nport-one", "--public"})

	err := cmd.Execute()

	assert.ErrorContains(t, err, "can only update an HTTP mapping")
}

func TestBuildPortUpdatesValidation(t *testing.T) {
	tests := []struct {
		name string
		opts updateOptions
		want string
	}{
		{name: "no updates", want: "specify at least one update"},
		{
			name: "allow conflicts with anywhere",
			opts: updateOptions{allowedSourcesSet: true, allowedSources: []string{"10.0.0.0/8"}, allowAnywhereSet: true, allowAnywhere: true},
			want: "--allow and --allow-anywhere cannot be used together",
		},
		{
			name: "authorize conflicts with public",
			opts: updateOptions{authorizedEmailsSet: true, authorizedEmails: []string{"me@example.com"}, publicSet: true, public: true},
			want: "--authorize and --public cannot be used together",
		},
		{
			name: "invalid destination",
			opts: updateOptions{destinationPortSet: true, destinationPort: "65536"},
			want: "must be a number between 1 and 65535",
		},
		{
			name: "invalid HTTP protocol",
			opts: updateOptions{protocolSet: true, protocol: "tcp"},
			want: "must be http or https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildPortUpdates(tt.opts)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}
