package ports

import (
	"bytes"
	"context"
	"errors"
	"testing"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/entity"
)

type fakeClosePrompter struct {
	selectIndex  int
	confirm      bool
	selectCalls  int
	confirmCalls int
	items        []string
}

func (p *fakeClosePrompter) Select(_ string, items []string) string {
	p.selectCalls++
	p.items = append([]string{}, items...)
	if p.selectIndex < 0 || p.selectIndex >= len(items) {
		return ""
	}
	return items[p.selectIndex]
}

func (p *fakeClosePrompter) ConfirmYesNo(_ string) bool {
	p.confirmCalls++
	return p.confirm
}

type fakeCloseEnvironmentService struct {
	devplanev1connect.UnimplementedEnvironmentServiceHandler
	t             *testing.T
	expectedEnvID string
	ports         []*devplanev1.Port
	closedPortIDs []string
	failPortID    string
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
	if req.Msg.GetPortId() == s.failPortID {
		return nil, connect.NewError(connect.CodeInternal, errors.New("close failed"))
	}
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

func TestCloseInteractivelySelectsOnePort(t *testing.T) {
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
	prompter := &fakeClosePrompter{selectIndex: 1, confirm: true}
	var out bytes.Buffer

	err := runClose(
		context.Background(),
		&out,
		newCloseEnvironmentStore(),
		prompter,
		"my-instance",
		closeOptions{},
	)

	require.NoError(t, err)
	assert.Equal(t, []string{"nport-two"}, service.closedPortIDs)
	assert.Equal(t, 1, prompter.selectCalls)
	assert.Equal(t, 1, prompter.confirmCalls)
	require.Len(t, prompter.items, 2)
	assert.Contains(t, prompter.items[0], "public 41001 -> destination 8080")
	assert.Contains(t, prompter.items[1], "public 52002 -> destination 8080")
	assert.Contains(t, out.String(), "Closed 1 port on my-instance.")
}

func TestCloseSelectionLabelMissingDestinationDoesNotUsePublicPort(t *testing.T) {
	label := closeSelectionLabel(0, &devplanev1.Port{
		Protocol:   devplanev1.PortProtocol_PORT_PROTOCOL_TCP,
		PortNumber: 443,
	})

	assert.Contains(t, label, "public 443 -> destination -")
	assert.NotContains(t, label, "destination 443")
}

func TestCloseByExactIDOnExternalNode(t *testing.T) {
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
	prompter := &fakeClosePrompter{selectIndex: -1}
	store := &fakeStore{
		user: &entity.User{ID: "user1"},
		org:  &entity.Organization{ID: "org1"},
	}
	var out bytes.Buffer

	err := runClose(
		context.Background(),
		&out,
		store,
		prompter,
		"unode123",
		closeOptions{portID: "nport-one", approve: true},
	)

	require.NoError(t, err)
	assert.Equal(t, []string{"nport-one"}, service.closedPortIDs)
	assert.Zero(t, prompter.selectCalls)
	assert.Zero(t, prompter.confirmCalls)
	assert.Contains(t, out.String(), "global.prd.ga.run.brev.nvidia.com:41001")
}

func TestCloseAllClosesSnapshot(t *testing.T) {
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
	prompter := &fakeClosePrompter{selectIndex: -1}
	var out bytes.Buffer

	err := runClose(
		context.Background(),
		&out,
		newCloseEnvironmentStore(),
		prompter,
		"my-instance",
		closeOptions{all: true, approve: true},
	)

	require.NoError(t, err)
	assert.Equal(t, []string{"nport-one", "nport-two"}, service.closedPortIDs)
	assert.Zero(t, prompter.selectCalls)
	assert.Zero(t, prompter.confirmCalls)
	assert.Contains(t, out.String(), "Closed 2 ports on my-instance.")
}

func TestCloseCancellationDoesNotClosePort(t *testing.T) {
	service := &fakeCloseEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		ports:         []*devplanev1.Port{testTCPPort("nport-one", 41001)},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)
	prompter := &fakeClosePrompter{selectIndex: 0, confirm: false}
	var out bytes.Buffer

	err := runClose(
		context.Background(),
		&out,
		newCloseEnvironmentStore(),
		prompter,
		"my-instance",
		closeOptions{},
	)

	require.NoError(t, err)
	assert.Empty(t, service.closedPortIDs)
	assert.Contains(t, out.String(), "No ports were closed.")
}

func TestCloseRejectsUnknownID(t *testing.T) {
	service := &fakeCloseEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		ports:         []*devplanev1.Port{testTCPPort("nport-one", 41001)},
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)
	var out bytes.Buffer

	err := runClose(
		context.Background(),
		&out,
		newCloseEnvironmentStore(),
		&fakeClosePrompter{},
		"my-instance",
		closeOptions{portID: "nport-missing", approve: true},
	)

	assert.ErrorContains(t, err, `port_id "nport-missing" is not open on this target`)
	assert.Empty(t, service.closedPortIDs)
}

func TestCloseAllReportsPartialFailure(t *testing.T) {
	service := &fakeCloseEnvironmentService{
		t:             t,
		expectedEnvID: "env123",
		ports: []*devplanev1.Port{
			testTCPPort("nport-one", 41001),
			testTCPPort("nport-two", 52002),
		},
		failPortID: "nport-one",
	}
	_, handler := devplanev1connect.NewEnvironmentServiceHandler(service)
	newTestServer(t, handler)
	var out bytes.Buffer

	err := runClose(
		context.Background(),
		&out,
		newCloseEnvironmentStore(),
		&fakeClosePrompter{},
		"my-instance",
		closeOptions{all: true, approve: true},
	)

	assert.ErrorContains(t, err, `close port_id "nport-one"`)
	assert.Equal(t, []string{"nport-two"}, service.closedPortIDs)
	assert.Contains(t, out.String(), "Closed 1 port on my-instance.")
}

func TestCloseCommandRejectsAllWithID(t *testing.T) {
	cmd := newCmdClosePort(newCloseEnvironmentStore(), &fakeClosePrompter{})
	cmd.SetArgs([]string{"my-instance", "--all", "--id", "nport-one"})

	err := cmd.Execute()

	assert.ErrorContains(t, err, "--all and --id cannot be used together")
}

func TestRemovablePortsRequiresPortID(t *testing.T) {
	got := removablePorts([]*devplanev1.Port{
		nil,
		{PortNumber: 1234},
		testTCPPort("nport-one", 41001),
	})

	require.Len(t, got, 1)
	assert.Equal(t, "nport-one", got[0].GetPortId())
}
