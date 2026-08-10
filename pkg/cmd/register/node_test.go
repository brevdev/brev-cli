package register

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/externalnode"
)

type registeredNodeTestFactory struct{ serverURL string }

func (f registeredNodeTestFactory) NewNodeClient(provider externalnode.TokenProvider, _ string) nodev1connect.ExternalNodeServiceClient {
	return NewNodeServiceClient(provider, f.serverURL)
}

type registeredNodeTestTokenProvider struct{}

func (registeredNodeTestTokenProvider) GetAccessToken() (string, error) { return "token", nil }

type registeredNodeTestService struct {
	nodev1connect.UnimplementedExternalNodeServiceHandler
	getNode func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error)
}

func (s registeredNodeTestService) GetNode(_ context.Context, req *connect.Request[nodev1.GetNodeRequest]) (*connect.Response[nodev1.GetNodeResponse], error) {
	resp, err := s.getNode(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func startRegisteredNodeTestServer(t *testing.T, service registeredNodeTestService) registeredNodeTestFactory {
	t.Helper()
	_, handler := nodev1connect.NewExternalNodeServiceHandler(service)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return registeredNodeTestFactory{serverURL: server.URL}
}

func TestFetchRegisteredNode_Success(t *testing.T) {
	factory := startRegisteredNodeTestServer(t, registeredNodeTestService{
		getNode: func(req *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			require.Equal(t, "unode_123", req.GetExternalNodeId())
			require.Equal(t, "org_456", req.GetOrganizationId())
			return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{ExternalNodeId: "unode_123"}}, nil
		},
	})

	node, err := FetchRegisteredNode(context.Background(), factory, registeredNodeTestTokenProvider{}, &DeviceRegistration{
		ExternalNodeID: "unode_123",
		OrgID:          "org_456",
	})

	require.NoError(t, err)
	require.Equal(t, "unode_123", node.GetExternalNodeId())
}

func TestFetchRegisteredNode_RPCError(t *testing.T) {
	factory := startRegisteredNodeTestServer(t, registeredNodeTestService{
		getNode: func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, errors.New("backend unavailable"))
		},
	})

	_, err := FetchRegisteredNode(context.Background(), factory, registeredNodeTestTokenProvider{}, &DeviceRegistration{
		ExternalNodeID: "unode_123",
		OrgID:          "org_456",
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "error retrieving joined node")
}

func TestFetchRegisteredNode_NilNodeIsError(t *testing.T) {
	tests := []struct {
		name string
		resp *connect.Response[nodev1.GetNodeResponse]
	}{
		{name: "nil response"},
		{name: "nil message", resp: &connect.Response[nodev1.GetNodeResponse]{}},
		{name: "nil node", resp: connect.NewResponse(&nodev1.GetNodeResponse{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := registeredNodeFromResponse(tt.resp)
			require.EqualError(t, err, `registered node was not returned by Brev; run "brev leave" and "brev join" to repair membership`)
		})
	}
}
