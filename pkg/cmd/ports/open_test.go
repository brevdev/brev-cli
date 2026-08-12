package ports

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/entity"
)

func newTestServer(t *testing.T, handler http.Handler) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)
}

type fakeOpenEnvironmentService struct {
	devplanev1connect.UnimplementedEnvironmentServiceHandler
	t       *testing.T
	wantReq *devplanev1.EnvironmentServiceOpenPortRequest
	port    *devplanev1.Port
}

func (s *fakeOpenEnvironmentService) OpenPort(
	_ context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceOpenPortRequest],
) (*connect.Response[devplanev1.EnvironmentServiceOpenPortResponse], error) {
	s.t.Helper()
	assert.Equal(s.t, s.wantReq.GetEnvironmentId(), req.Msg.GetEnvironmentId())
	assert.Equal(s.t, s.wantReq.GetPortNumber(), req.Msg.GetPortNumber())
	assert.Equal(s.t, s.wantReq.GetProtocol(), req.Msg.GetProtocol())
	assert.Equal(s.t, s.wantReq.GetAllowedSources(), req.Msg.GetAllowedSources())
	return connect.NewResponse(&devplanev1.EnvironmentServiceOpenPortResponse{Port: s.port}), nil
}

type fakeOpenNodeService struct {
	devplanev1connect.UnimplementedExternalNodeServiceHandler
	t       *testing.T
	node    *devplanev1.ExternalNode
	wantReq *devplanev1.OpenPortRequest
	port    *devplanev1.Port
}

func (s *fakeOpenNodeService) ListNodes(
	_ context.Context,
	_ *connect.Request[devplanev1.ListNodesRequest],
) (*connect.Response[devplanev1.ListNodesResponse], error) {
	return connect.NewResponse(&devplanev1.ListNodesResponse{Items: []*devplanev1.ExternalNode{s.node}}), nil
}

func (s *fakeOpenNodeService) OpenPort(
	_ context.Context,
	req *connect.Request[devplanev1.OpenPortRequest],
) (*connect.Response[devplanev1.OpenPortResponse], error) {
	s.t.Helper()
	assert.Equal(s.t, s.wantReq.GetExternalNodeId(), req.Msg.GetExternalNodeId())
	assert.Equal(s.t, s.wantReq.GetPortNumber(), req.Msg.GetPortNumber())
	assert.Equal(s.t, s.wantReq.GetProtocol(), req.Msg.GetProtocol())
	assert.Equal(s.t, s.wantReq.GetAllowedSources(), req.Msg.GetAllowedSources())
	return connect.NewResponse(&devplanev1.OpenPortResponse{Port: s.port}), nil
}

func TestOpenEnvironment(t *testing.T) {
	service := &fakeOpenEnvironmentService{
		t: t,
		wantReq: &devplanev1.EnvironmentServiceOpenPortRequest{
			EnvironmentId:  "env123",
			Protocol:       devplanev1.PortProtocol_PORT_PROTOCOL_TCP,
			PortNumber:     8080,
			AllowedSources: []string{"203.0.113.10/32"},
		},
		port: &devplanev1.Port{
			PortId:         "port123",
			Protocol:       devplanev1.PortProtocol_PORT_PROTOCOL_TCP,
			PortNumber:     19001,
			ServerPort:     8080,
			AllowedSources: []string{"203.0.113.10/32"},
		},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)

	store := &fakeStore{
		workspaces: []entity.Workspace{{ID: "env123", Name: "my-instance", CreatedByUserID: "user1"}},
		user:       &entity.User{ID: "user1"},
		org:        &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := Open(
		context.Background(),
		&out,
		store,
		"my-instance",
		8080,
		devplanev1.PortProtocol_PORT_PROTOCOL_TCP,
		[]string{"203.0.113.10/32"},
		false,
	)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Opened TCP port 8080 on my-instance.")
	assert.Contains(t, out.String(), "19001")
	assert.Contains(t, out.String(), "203.0.113.10/32")
}

func TestOpenExternalNodeByIDJSON(t *testing.T) {
	hostname := "global.prd.ga.run.brev.nvidia.com"
	service := &fakeOpenNodeService{
		t:    t,
		node: &devplanev1.ExternalNode{ExternalNodeId: "unode123", Name: "my-node"},
		wantReq: &devplanev1.OpenPortRequest{
			ExternalNodeId: "unode123",
			Protocol:       devplanev1.PortProtocol_PORT_PROTOCOL_UDP,
			PortNumber:     53,
		},
		port: &devplanev1.Port{
			PortId:     "port53",
			Protocol:   devplanev1.PortProtocol_PORT_PROTOCOL_UDP,
			PortNumber: 19053,
			ServerPort: 53,
			Hostname:   &hostname,
		},
	}
	_, handler := devplanev1connect.NewExternalNodeServiceHandler(service)
	newTestServer(t, handler)

	store := &fakeStore{
		user: &entity.User{ID: "user1"},
		org:  &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := Open(
		context.Background(),
		&out,
		store,
		"unode123",
		53,
		devplanev1.PortProtocol_PORT_PROTOCOL_UDP,
		nil,
		true,
	)

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"port_id": "port53",
		"kind": "tcp_udp",
		"endpoint": "global.prd.ga.run.brev.nvidia.com:19053",
		"public_port": 19053,
		"destination_port": 53,
		"protocol": "UDP",
		"allowed_sources": [],
		"authorized_emails": [],
		"allow_public_unauthenticated": false,
		"type": "unspecified"
	}`, out.String())
}

func TestNewCmdOpenPortParsesFlags(t *testing.T) {
	service := &fakeOpenEnvironmentService{
		t: t,
		wantReq: &devplanev1.EnvironmentServiceOpenPortRequest{
			EnvironmentId:  "env123",
			Protocol:       devplanev1.PortProtocol_PORT_PROTOCOL_SSH,
			PortNumber:     2222,
			AllowedSources: []string{"10.0.0.0/8", "192.0.2.0/24"},
		},
		port: &devplanev1.Port{
			PortId:     "port2222",
			Protocol:   devplanev1.PortProtocol_PORT_PROTOCOL_SSH,
			PortNumber: 19222,
			ServerPort: 2222,
		},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)

	store := &fakeStore{
		workspaces: []entity.Workspace{{ID: "env123", Name: "my-instance", CreatedByUserID: "user1"}},
		user:       &entity.User{ID: "user1"},
		org:        &entity.Organization{ID: "org1"},
	}
	cmd := NewCmdPorts(store)
	cmd.SetArgs([]string{
		"open", "my-instance", "2222", "--protocol", "SSH",
		"--allow", "10.0.0.0/8", "--allow", "192.0.2.0/24",
		"--allow", "10.0.0.0/8",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Opened SSH port 2222 on my-instance.")
}

func TestParsePortNumber(t *testing.T) {
	tests := []struct {
		value string
		want  int32
		err   bool
	}{
		{value: "1", want: 1},
		{value: "65535", want: 65535},
		{value: "0", err: true},
		{value: "65536", err: true},
		{value: "http", err: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := parsePortNumber(tt.value)
			if tt.err {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseProtocol(t *testing.T) {
	tests := []struct {
		input string
		want  devplanev1.PortProtocol
	}{
		{input: "tcp", want: devplanev1.PortProtocol_PORT_PROTOCOL_TCP},
		{input: "UDP", want: devplanev1.PortProtocol_PORT_PROTOCOL_UDP},
		{input: " ssh ", want: devplanev1.PortProtocol_PORT_PROTOCOL_SSH},
	}
	for _, tt := range tests {
		got, err := parseProtocol(tt.input)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got)
	}

	_, err := parseProtocol("http")
	assert.EqualError(t, err, `invalid protocol "http": must be tcp, udp, or ssh`)
}
