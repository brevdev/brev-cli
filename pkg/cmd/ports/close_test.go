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

type fakeCloseEnvironmentService struct {
	devplanev1connect.UnimplementedEnvironmentServiceHandler
	t             *testing.T
	expectedEnvID string
	ports         []*devplanev1.Port
	closedPortIDs []string
}

func (s *fakeCloseEnvironmentService) GetNetworkInfo(
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

func (s *fakeCloseEnvironmentService) ClosePort(
	_ context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceClosePortRequest],
) (*connect.Response[devplanev1.EnvironmentServiceClosePortResponse], error) {
	s.t.Helper()
	s.closedPortIDs = append(s.closedPortIDs, req.Msg.GetPortId())
	return connect.NewResponse(&devplanev1.EnvironmentServiceClosePortResponse{}), nil
}

type fakeCloseNodeService struct {
	devplanev1connect.UnimplementedExternalNodeServiceHandler
	t             *testing.T
	node          *devplanev1.ExternalNode
	closedPortIDs []string
}

func (s *fakeCloseNodeService) ListNodes(
	_ context.Context,
	_ *connect.Request[devplanev1.ListNodesRequest],
) (*connect.Response[devplanev1.ListNodesResponse], error) {
	return connect.NewResponse(&devplanev1.ListNodesResponse{
		Items: []*devplanev1.ExternalNode{s.node},
	}), nil
}

func (s *fakeCloseNodeService) ClosePort(
	_ context.Context,
	req *connect.Request[devplanev1.ClosePortRequest],
) (*connect.Response[devplanev1.ClosePortResponse], error) {
	s.t.Helper()
	s.closedPortIDs = append(s.closedPortIDs, req.Msg.GetPortId())
	return connect.NewResponse(&devplanev1.ClosePortResponse{}), nil
}

func newCloseEnvironmentStore() *fakeStore {
	return &fakeStore{
		workspaces: []entity.Workspace{{ID: "env123", Name: "my-instance", CreatedByUserID: "user1"}},
		user:       &entity.User{ID: "user1"},
		org:        &entity.Organization{ID: "org1"},
	}
}

func testTCPPort(id string, publicPort int32) *devplanev1.Port {
	hostname := "global.prd.ga.run.brev.nvidia.com"
	return &devplanev1.Port{
		PortId:     id,
		Protocol:   devplanev1.PortProtocol_PORT_PROTOCOL_TCP,
		PortNumber: publicPort,
		ServerPort: 8080,
		Hostname:   &hostname,
		Type:       devplanev1.PortType_PORT_TYPE_USER,
	}
}

func TestRemoveByDestinationPort(t *testing.T) {
	second := testTCPPort("nport-two", 52002)
	second.ServerPort = 9090
	service := &fakeCloseEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		ports: []*devplanev1.Port{
			testTCPPort("nport-one", 41001),
			second,
		},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)
	var out bytes.Buffer

	err := runRemoveByDestination(
		context.Background(),
		&out,
		newCloseEnvironmentStore(),
		"my-instance",
		"9090",
	)

	require.NoError(t, err)
	assert.Equal(t, []string{"nport-two"}, service.closedPortIDs)
	assert.Equal(t, "Removed TCP port 9090 on my-instance.\n", out.String())
}

func TestRemoveByExactIDOnBrevConnectMachine(t *testing.T) {
	service := &fakeCloseNodeService{
		t: t,
		node: &devplanev1.ExternalNode{
			ExternalNodeId: "unode123",
			Name:           "my-node",
			Ports: []*devplanev1.Port{
				testTCPPort("nport-one", 41001),
				testTCPPort("nport-two", 52002),
			},
		},
	}
	_, handler := devplanev1connect.NewExternalNodeServiceHandler(service)
	newTestServer(t, handler)
	store := &fakeStore{
		user: &entity.User{ID: "user1"},
		org:  &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := runRemoveByID(
		context.Background(),
		&out,
		store,
		"nport-one",
	)

	require.NoError(t, err)
	assert.Equal(t, []string{"nport-one"}, service.closedPortIDs)
	assert.Equal(t, "Removed TCP port 8080 on my-node.\n", out.String())
}

func TestRemoveRejectsUnknownDestination(t *testing.T) {
	service := &fakeCloseEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		ports:         []*devplanev1.Port{testTCPPort("nport-one", 41001)},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)

	err := runRemoveByDestination(context.Background(), &bytes.Buffer{}, newCloseEnvironmentStore(), "my-instance", "9090")

	assert.ErrorContains(t, err, "destination port 9090 is not open on this target")
	assert.Empty(t, service.closedPortIDs)
}

func TestRemoveRejectsAmbiguousDestination(t *testing.T) {
	service := &fakeCloseEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		ports: []*devplanev1.Port{
			testTCPPort("nport-one", 41001),
			testTCPPort("nport-two", 52002),
		},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)

	err := runRemoveByDestination(context.Background(), &bytes.Buffer{}, newCloseEnvironmentStore(), "my-instance", "8080")

	assert.ErrorContains(t, err, "destination port 8080 matches multiple ports (nport-one, nport-two); use an exact port_id from `brev ports ls`")
	assert.Empty(t, service.closedPortIDs)
}
