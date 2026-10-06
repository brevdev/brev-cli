package ports

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/entity"
)

func testGetPort() *devplanev1.Port {
	hostname := "app.example.com"
	public := true
	return &devplanev1.Port{
		PortId:                     "nport-abc123",
		HttpProtocol:               devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTPS,
		PortNumber:                 443,
		ServerPort:                 8443,
		Hostname:                   &hostname,
		AllowedSources:             []string{"203.0.113.10/32"},
		AuthorizedEmails:           []string{"user@example.com"},
		AllowPublicUnauthenticated: &public,
		Type:                       devplanev1.PortType_PORT_TYPE_USER,
	}
}

func newGetEnvironmentStore() *fakeStore {
	return &fakeStore{
		workspaces: []entity.Workspace{{ID: "env123", Name: "my-instance", CreatedByUserID: "user1"}},
		user:       &entity.User{ID: "user1"},
		org:        &entity.Organization{ID: "org1"},
	}
}

func serveGetPort(t *testing.T) {
	t.Helper()
	service := &fakeEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		networkInfo: &devplanev1.EnvironmentNetworkInfo{
			Status: devplanev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_CONNECTED,
			Ports:  []*devplanev1.Port{testGetPort()},
		},
	}
	environmentPath, environmentHandler := devplanev1connect.NewEnvironmentServiceHandler(service)
	nodePath, nodeHandler := devplanev1connect.NewExternalNodeServiceHandler(&fakeNodeService{})
	mux := http.NewServeMux()
	mux.Handle(environmentPath, environmentHandler)
	mux.Handle(nodePath, nodeHandler)
	newTestServer(t, mux)
}

func TestGetDisplaysAllPortData(t *testing.T) {
	serveGetPort(t)
	var out bytes.Buffer

	err := Get(context.Background(), &out, newGetEnvironmentStore(), "nport-abc123", false)

	require.NoError(t, err)
	for _, value := range []string{
		"ID", "nport-abc123",
		"ENDPOINT", "https://app.example.com",
		"PUBLIC PORT", "443",
		"DESTINATION PORT", "8443",
		"PROTOCOL", "HTTPS",
		"ALLOWED SOURCES", "203.0.113.10/32",
		"AUTHORIZED EMAILS", "user@example.com",
		"PUBLIC UNAUTHENTICATED", "true",
		"TYPE", "user",
	} {
		assert.Contains(t, out.String(), value)
	}
}

func TestGetJSONIsSingleCompletePort(t *testing.T) {
	serveGetPort(t)
	var out bytes.Buffer

	err := Get(context.Background(), &out, newGetEnvironmentStore(), "nport-abc123", true)

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"port_id":"nport-abc123",
		"endpoint":"https://app.example.com",
		"public_port":443,
		"destination_port":8443,
		"protocol":"HTTPS",
		"allowed_sources":["203.0.113.10/32"],
		"authorized_emails":["user@example.com"],
		"allow_public_unauthenticated":true,
		"type":"user"
	}`, out.String())
}

func TestGetRejectsUnknownPortID(t *testing.T) {
	serveGetPort(t)

	err := Get(context.Background(), &bytes.Buffer{}, newGetEnvironmentStore(), "missing", false)

	assert.ErrorContains(t, err, `port_id "missing" is not open in the active organization`)
}
