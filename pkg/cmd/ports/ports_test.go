package ports

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/entity"
)

type fakeStore struct {
	workspaces []entity.Workspace
	user       *entity.User
	org        *entity.Organization
}

func (s *fakeStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return nil, nil
}

func (s *fakeStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return s.org, nil
}

func (s *fakeStore) GetWorkspaceByNameOrID(_ string, _ string) ([]entity.Workspace, error) {
	return s.workspaces, nil
}

func (s *fakeStore) GetCurrentUser() (*entity.User, error) {
	return s.user, nil
}

func (s *fakeStore) GetAccessToken() (string, error) {
	return "test-token", nil
}

type fakeEnvironmentService struct {
	devplanev1connect.UnimplementedEnvironmentServiceHandler
	t             *testing.T
	expectedEnvID string
	networkInfo   *devplanev1.EnvironmentNetworkInfo
}

func (s *fakeEnvironmentService) GetNetworkInfo(
	_ context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceGetNetworkInfoRequest],
) (*connect.Response[devplanev1.EnvironmentServiceGetNetworkInfoResponse], error) {
	s.t.Helper()
	assert.Equal(s.t, s.expectedEnvID, req.Msg.GetEnvironmentId())
	return connect.NewResponse(&devplanev1.EnvironmentServiceGetNetworkInfoResponse{
		NetworkInfo: s.networkInfo,
	}), nil
}

type fakeNodeService struct {
	devplanev1connect.UnimplementedExternalNodeServiceHandler
	nodes []*devplanev1.ExternalNode
}

func (s *fakeNodeService) ListNodes(
	_ context.Context,
	_ *connect.Request[devplanev1.ListNodesRequest],
) (*connect.Response[devplanev1.ListNodesResponse], error) {
	return connect.NewResponse(&devplanev1.ListNodesResponse{Items: s.nodes}), nil
}

func TestPortsCommandUsesSubcommands(t *testing.T) {
	cmd := NewCmdPorts(&fakeStore{})
	assert.Equal(t, "[beta] Manage ports for an instance or external node", cmd.Short)

	listCmd, _, err := cmd.Find([]string{"ls"})
	require.NoError(t, err)
	assert.Equal(t, "ls <instance-or-node>", listCmd.Use)
	assert.Equal(t, "[beta] List Brev-managed ports for an instance or external node", listCmd.Short)
	assert.Contains(t, listCmd.Annotations, "access")
	assert.Nil(t, cmd.Flags().Lookup("json"))
	assert.NotNil(t, listCmd.Flags().Lookup("json"))

	createCmd, _, err := cmd.Find([]string{"create"})
	require.NoError(t, err)
	assert.Equal(t, "create <instance-or-node> <port>", createCmd.Use)
	assert.Equal(t, "[beta] Create a public port on an instance or external node", createCmd.Short)
	assert.ElementsMatch(t, []string{"open", "add"}, createCmd.Aliases)
}

func TestRunEnvironmentJSON(t *testing.T) {
	public := false
	hostname := "jupyter-env123.apps.run.brev.nvidia.com"
	service := &fakeEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		networkInfo: &devplanev1.EnvironmentNetworkInfo{
			Status: devplanev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_CONNECTED,
			Ports: []*devplanev1.Port{
				{
					PortId:                     "port-http",
					HttpProtocol:               devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTP,
					PortNumber:                 443,
					ServerPort:                 8888,
					Hostname:                   &hostname,
					AuthorizedEmails:           []string{"user@example.com"},
					AllowPublicUnauthenticated: &public,
					Type:                       devplanev1.PortType_PORT_TYPE_SYSTEM,
				},
			},
		},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)

	store := &fakeStore{
		workspaces: []entity.Workspace{{ID: "env123", Name: "my-instance", CreatedByUserID: "user1"}},
		user:       &entity.User{ID: "user1"},
		org:        &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := Run(context.Background(), &out, store, "my-instance", true)

	require.NoError(t, err)
	assert.JSONEq(t, `[
		{
			"port_id": "port-http",
			"endpoint": "https://jupyter-env123.apps.run.brev.nvidia.com",
			"public_port": 443,
			"destination_port": 8888,
			"protocol": "HTTP",
			"allowed_sources": [],
			"authorized_emails": ["user@example.com"],
			"allow_public_unauthenticated": false,
			"type": "system"
		}
	]`, out.String())
}

func TestRunExternalNodeByIDDisplaysTables(t *testing.T) {
	httpHostname := "jupyter-node.apps.run.brev.nvidia.com"
	tcpHostname := "global.prd.ga.run.brev.nvidia.com"
	service := &fakeNodeService{nodes: []*devplanev1.ExternalNode{
		{
			ExternalNodeId: "unode123",
			Name:           "my-node",
			Ports: []*devplanev1.Port{
				{
					PortId:           "port-http",
					HttpProtocol:     devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTP,
					PortNumber:       443,
					ServerPort:       8888,
					Hostname:         &httpHostname,
					AuthorizedEmails: []string{"user@example.com"},
				},
				{
					PortId:         "port-tcp",
					Protocol:       devplanev1.PortProtocol_PORT_PROTOCOL_TCP,
					PortNumber:     18928,
					ServerPort:     22,
					Hostname:       &tcpHostname,
					AllowedSources: []string{"0.0.0.0/0"},
				},
			},
		},
	}}
	_, handler := devplanev1connect.NewExternalNodeServiceHandler(service)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)

	store := &fakeStore{
		user: &entity.User{ID: "user1"},
		org:  &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := Run(context.Background(), &out, store, "unode123", false)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "HTTP APPLICATIONS")
	assert.Contains(t, out.String(), "https://jupyter-node.apps.run.brev.nvidia.com")
	assert.Contains(t, out.String(), "user@example.com")
	assert.Contains(t, out.String(), "NETWORK PORTS")
	assert.Contains(t, out.String(), "PUBLIC PORT")
	assert.Contains(t, out.String(), "DESTINATION PORT")
	assert.Contains(t, out.String(), "global.prd.ga.run.brev.nvidia.com:18928")
	assert.Contains(t, out.String(), "Anywhere")
	assert.Contains(t, out.String(), "22")
	assert.Contains(t, out.String(), "TCP")
}

func TestDisplayTablesSSHUsesNetworkHeading(t *testing.T) {
	var out bytes.Buffer
	ports := []PortInfo{
		{
			Endpoint:        "gateway.example.com:18928",
			PublicPort:      18928,
			DestinationPort: 22,
			Protocol:        "SSH",
		},
	}

	err := displayTables(&out, "ssh-node", ports)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "NETWORK PORTS")
	assert.Contains(t, out.String(), "gateway.example.com:18928")
	assert.Contains(t, out.String(), "SSH")
	assert.NotContains(t, out.String(), "TCP/UDP PORTS")
}

func TestDisplayHTTPTableMissingDestinationDoesNotUsePublicPort(t *testing.T) {
	var out bytes.Buffer

	displayHTTPTable(&out, []PortInfo{{
		Endpoint:   "https://app.example.com",
		PublicPort: 443,
		Protocol:   "HTTP",
	}})

	assert.Regexp(t, `443\s+-\s+HTTP`, out.String())
	assert.NotRegexp(t, `443\s+443\s+HTTP`, out.String())
}

func TestToPortInfosHandlesPublicHTTPAndRestrictedUDP(t *testing.T) {
	public := true
	httpHostname := "app.example.com"
	udpHostname := "gateway.example.com"

	got := toPortInfos([]*devplanev1.Port{
		{
			PortId:                     "http",
			HttpProtocol:               devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTPS,
			PortNumber:                 443,
			ServerPort:                 8443,
			Hostname:                   &httpHostname,
			AllowPublicUnauthenticated: &public,
		},
		{
			PortId:         "udp",
			Protocol:       devplanev1.PortProtocol_PORT_PROTOCOL_UDP,
			PortNumber:     5000,
			ServerPort:     5001,
			Hostname:       &udpHostname,
			AllowedSources: []string{"10.0.0.0/8"},
			Type:           devplanev1.PortType_PORT_TYPE_USER,
		},
		nil,
	})

	require.Len(t, got, 2)
	assert.True(t, got[0].isHTTP)
	assert.Equal(t, "HTTPS", got[0].Protocol)
	assert.Equal(t, "https://app.example.com", got[0].Endpoint)
	assert.True(t, got[0].AllowPublicUnauthenticated)
	assert.False(t, got[1].isHTTP)
	assert.Equal(t, "UDP", got[1].Protocol)
	assert.Equal(t, "gateway.example.com:5000", got[1].Endpoint)
	assert.Equal(t, []string{"10.0.0.0/8"}, got[1].AllowedSources)
	assert.Equal(t, "user", got[1].Type)
	assert.Equal(t, "unspecified", portTypeLabel(devplanev1.PortType_PORT_TYPE_UNSPECIFIED))
	assert.Equal(t, "unknown", portTypeLabel(devplanev1.PortType(99)))
}

func TestRunEmptyPortsJSONIsArray(t *testing.T) {
	service := &fakeEnvironmentService{
		t:             t,
		expectedEnvID: "env-empty",
		networkInfo: &devplanev1.EnvironmentNetworkInfo{
			Status: devplanev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_DISCONNECTED,
		},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)

	store := &fakeStore{
		workspaces: []entity.Workspace{{ID: "env-empty", Name: "empty", CreatedByUserID: "user1"}},
		user:       &entity.User{ID: "user1"},
		org:        &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := Run(context.Background(), &out, store, "empty", true)

	require.NoError(t, err)
	assert.JSONEq(t, `[]`, out.String())
}

func TestRunEmptyPortsHumanOutput(t *testing.T) {
	service := &fakeEnvironmentService{
		t:             t,
		expectedEnvID: "env-empty",
		networkInfo: &devplanev1.EnvironmentNetworkInfo{
			Status: devplanev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_CONNECTED,
		},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)

	store := &fakeStore{
		workspaces: []entity.Workspace{{ID: "env-empty", Name: "empty", CreatedByUserID: "user1"}},
		user:       &entity.User{ID: "user1"},
		org:        &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := Run(context.Background(), &out, store, "empty", false)

	require.NoError(t, err)
	assert.Equal(t, "No ports are open on empty.\n", out.String())
}

func TestRunEnvironmentWithoutNetworkMemberReturnsActionableError(t *testing.T) {
	testCases := map[string]*devplanev1.EnvironmentNetworkInfo{
		"missing network info": nil,
		"unspecified status":   {},
	}
	for name, networkInfo := range testCases {
		t.Run(name, func(t *testing.T) {
			service := &fakeEnvironmentService{
				t:             t,
				expectedEnvID: "env-legacy",
				networkInfo:   networkInfo,
			}
			_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
			server := httptest.NewServer(handler)
			t.Cleanup(server.Close)
			t.Setenv("BREV_PUBLIC_API_URL", server.URL)

			store := &fakeStore{
				workspaces: []entity.Workspace{{ID: "env-legacy", Name: "legacy", CreatedByUserID: "user1"}},
				user:       &entity.User{ID: "user1"},
				org:        &entity.Organization{ID: "org1"},
			}
			var out bytes.Buffer

			err := Run(context.Background(), &out, store, "legacy", true)

			require.Error(t, err)
			assert.Empty(t, out.String())
			assert.Contains(t, err.Error(), "no Brev-managed network configuration is available")
			assert.Contains(t, err.Error(), "may still be provisioning or may use legacy network access")
			assert.Contains(t, err.Error(), "Brev console")
		})
	}
}

func TestRunEnvironmentWithPortsAndUnspecifiedStatusStillLists(t *testing.T) {
	hostname := "app.example.com"
	service := &fakeEnvironmentService{
		t:             t,
		expectedEnvID: "env-partial",
		networkInfo: &devplanev1.EnvironmentNetworkInfo{
			Ports: []*devplanev1.Port{
				{
					PortId:       "port-http",
					HttpProtocol: devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTP,
					PortNumber:   443,
					ServerPort:   8080,
					Hostname:     &hostname,
				},
			},
		},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)

	store := &fakeStore{
		workspaces: []entity.Workspace{{ID: "env-partial", Name: "partial", CreatedByUserID: "user1"}},
		user:       &entity.User{ID: "user1"},
		org:        &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := Run(context.Background(), &out, store, "partial", true)

	require.NoError(t, err)
	assert.Contains(t, out.String(), `"port_id": "port-http"`)
}

func TestRunExternalNodeJSONContract(t *testing.T) {
	hostname := "global.prd.ga.run.brev.nvidia.com"
	service := &fakeNodeService{nodes: []*devplanev1.ExternalNode{
		{
			ExternalNodeId: "unode-json",
			Name:           "json-node",
			Ports: []*devplanev1.Port{
				{
					PortId:         "port-ssh",
					Protocol:       devplanev1.PortProtocol_PORT_PROTOCOL_SSH,
					PortNumber:     18928,
					ServerPort:     22,
					Hostname:       &hostname,
					AllowedSources: []string{"10.0.0.0/8"},
					Type:           devplanev1.PortType_PORT_TYPE_USER,
				},
			},
		},
	}}
	_, handler := devplanev1connect.NewExternalNodeServiceHandler(service)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)

	store := &fakeStore{
		user: &entity.User{ID: "user1"},
		org:  &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := Run(context.Background(), &out, store, "unode-json", true)

	require.NoError(t, err)
	assert.JSONEq(t, `[
		{
			"port_id": "port-ssh",
			"endpoint": "global.prd.ga.run.brev.nvidia.com:18928",
			"public_port": 18928,
			"destination_port": 22,
			"protocol": "SSH",
			"allowed_sources": ["10.0.0.0/8"],
			"authorized_emails": [],
			"allow_public_unauthenticated": false,
			"type": "user"
		}
	]`, out.String())
}
