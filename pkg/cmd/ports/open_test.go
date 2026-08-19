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
	t           *testing.T
	wantReq     *devplanev1.EnvironmentServiceOpenPortRequest
	wantHTTPReq *devplanev1.EnvironmentServiceOpenHTTPPortRequest
	port        *devplanev1.Port
	httpPort    *devplanev1.Port
}

type httpOpenRequest interface {
	GetPortNumber() int32
	GetCustomHostname() string
	GetHttpProtocol() devplanev1.HttpPortProtocol
	GetAuthorizedEmails() []string
	GetAllowPublicUnauthenticated() bool
}

func assertHTTPOpenRequest(t *testing.T, want, got httpOpenRequest) {
	t.Helper()
	assert.Equal(t, want.GetPortNumber(), got.GetPortNumber())
	assert.Equal(t, want.GetCustomHostname(), got.GetCustomHostname())
	assert.Equal(t, want.GetHttpProtocol(), got.GetHttpProtocol())
	assert.Equal(t, want.GetAuthorizedEmails(), got.GetAuthorizedEmails())
	assert.Equal(t, want.GetAllowPublicUnauthenticated(), got.GetAllowPublicUnauthenticated())
}

func (s *fakeOpenEnvironmentService) OpenHTTPPort(
	_ context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceOpenHTTPPortRequest],
) (*connect.Response[devplanev1.EnvironmentServiceOpenHTTPPortResponse], error) {
	s.t.Helper()
	assert.Equal(s.t, s.wantHTTPReq.GetEnvironmentId(), req.Msg.GetEnvironmentId())
	assertHTTPOpenRequest(s.t, s.wantHTTPReq, req.Msg)
	return connect.NewResponse(&devplanev1.EnvironmentServiceOpenHTTPPortResponse{Port: s.httpPort}), nil
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
	t           *testing.T
	node        *devplanev1.ExternalNode
	wantReq     *devplanev1.OpenPortRequest
	wantHTTPReq *devplanev1.OpenHTTPPortRequest
	port        *devplanev1.Port
	httpPort    *devplanev1.Port
}

func (s *fakeOpenNodeService) OpenHTTPPort(
	_ context.Context,
	req *connect.Request[devplanev1.OpenHTTPPortRequest],
) (*connect.Response[devplanev1.OpenHTTPPortResponse], error) {
	s.t.Helper()
	assert.Equal(s.t, s.wantHTTPReq.GetExternalNodeId(), req.Msg.GetExternalNodeId())
	assertHTTPOpenRequest(s.t, s.wantHTTPReq, req.Msg)
	return connect.NewResponse(&devplanev1.OpenHTTPPortResponse{Port: s.httpPort}), nil
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
	assert.Contains(t, out.String(), "Created TCP port 8080 on my-instance.")
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

func TestOpenHTTPEnvironmentDefaultsToCurrentUser(t *testing.T) {
	hostname := "3000-env123.apps.run.brev.nvidia.com"
	service := &fakeOpenEnvironmentService{
		t: t,
		wantHTTPReq: &devplanev1.EnvironmentServiceOpenHTTPPortRequest{
			EnvironmentId:    "env123",
			PortNumber:       3000,
			CustomHostname:   "3000-env123",
			HttpProtocol:     devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTP,
			AuthorizedEmails: []string{"me@example.com"},
		},
		httpPort: &devplanev1.Port{
			PortId:           "http3000",
			HttpProtocol:     devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTP,
			PortNumber:       443,
			ServerPort:       3000,
			Hostname:         &hostname,
			AuthorizedEmails: []string{"me@example.com"},
		},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)
	store := &fakeStore{
		workspaces: []entity.Workspace{{ID: "env123", Name: "my-instance", CreatedByUserID: "user1"}},
		user:       &entity.User{ID: "user1", Email: "me@example.com"},
		org:        &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := OpenHTTP(
		context.Background(),
		&out,
		store,
		"my-instance",
		3000,
		devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTP,
		"",
		nil,
		false,
		false,
	)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Created HTTP port 3000 on my-instance.")
	assert.Contains(t, out.String(), "https://3000-env123.apps.run.brev.nvidia.com")
	assert.Contains(t, out.String(), "me@example.com")
}

func TestOpenHTTPExternalNodePublicJSON(t *testing.T) {
	hostname := "demo-unode123.apps.run.brev.nvidia.com"
	public := true
	service := &fakeOpenNodeService{
		t:    t,
		node: &devplanev1.ExternalNode{ExternalNodeId: "unode123", Name: "my-node"},
		wantHTTPReq: &devplanev1.OpenHTTPPortRequest{
			ExternalNodeId:             "unode123",
			PortNumber:                 8443,
			CustomHostname:             "demo-unode123",
			HttpProtocol:               devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTPS,
			AllowPublicUnauthenticated: true,
		},
		httpPort: &devplanev1.Port{
			PortId:                     "http8443",
			HttpProtocol:               devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTPS,
			PortNumber:                 443,
			ServerPort:                 8443,
			Hostname:                   &hostname,
			AllowPublicUnauthenticated: &public,
		},
	}
	_, handler := devplanev1connect.NewExternalNodeServiceHandler(service)
	newTestServer(t, handler)
	store := &fakeStore{
		user: &entity.User{ID: "user1", Email: "me@example.com"},
		org:  &entity.Organization{ID: "org1"},
	}
	cmd := NewCmdPorts(store)
	cmd.SetArgs([]string{
		"create", "my-node", "8443", "--protocol", "HTTPS",
		"--hostname", "demo", "--public", "--json",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"port_id": "http8443",
		"endpoint": "https://demo-unode123.apps.run.brev.nvidia.com",
		"public_port": 443,
		"destination_port": 8443,
		"protocol": "HTTPS",
		"allowed_sources": [],
		"authorized_emails": [],
		"allow_public_unauthenticated": true,
		"type": "unspecified"
	}`, out.String())
}

func TestNewCmdCreatePortParsesFlags(t *testing.T) {
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
		"create", "my-instance", "2222", "--protocol", "SSH",
		"--allow", "10.0.0.0/8", "--allow", "192.0.2.0/24",
		"--allow", "10.0.0.0/8",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Created SSH port 2222 on my-instance.")
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
	assert.EqualError(t, err, `invalid protocol "http": must be tcp, udp, ssh, http, or https`)
}

func TestOpenHTTPFlagValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "public with authorized email",
			args: []string{"create", "my-instance", "8080", "--protocol", "http", "--public", "--authorize", "me@example.com"},
			want: "--public and --authorize cannot be used together",
		},
		{
			name: "IP allow-list on HTTP",
			args: []string{"create", "my-instance", "8080", "--protocol", "http", "--allow", "10.0.0.0/8"},
			want: "--allow is only supported for tcp, udp, and ssh ports",
		},
		{
			name: "HTTP flag on TCP",
			args: []string{"create", "my-instance", "8080", "--public"},
			want: "--hostname, --authorize, and --public are only supported for http and https ports",
		},
		{
			name: "invalid hostname",
			args: []string{"create", "my-instance", "8080", "--protocol", "http", "--hostname", "Not Valid"},
			want: "hostname must contain only lowercase letters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCmdPorts(&fakeStore{})
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			assert.ErrorContains(t, err, tt.want)
		})
	}
}

func TestBuildHTTPHostname(t *testing.T) {
	assert.Equal(t, "8080-env123", buildHTTPHostname("", 8080, "ENV123"))
	assert.Equal(t, "demo-env123", buildHTTPHostname(" demo ", 8080, "ENV123"))
	assert.Equal(t, "demo-env123", buildHTTPHostname("demo-env123", 8080, "ENV123"))
}
