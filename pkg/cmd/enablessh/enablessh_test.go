package enablessh

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type mockNodeClientFactory struct{ serverURL string }

func (m mockNodeClientFactory) NewNodeClient(provider externalnode.TokenProvider, _ string) nodev1connect.ExternalNodeServiceClient {
	return register.NewNodeServiceClient(provider, m.serverURL)
}

type mockEnableSSHStore struct {
	token string
	user  *entity.User
	err   error
}

func (m *mockEnableSSHStore) GetCurrentUser() (*entity.User, error) { return m.user, m.err }
func (m *mockEnableSSHStore) GetAccessToken() (string, error)       { return m.token, nil }

// fakeNodeService implements the server side of ExternalNodeService for testing.
type fakeNodeService struct {
	nodev1connect.UnimplementedExternalNodeServiceHandler
	getNodeFn    func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error)
	order        *[]string
	addNodeCalls int
}

func (f *fakeNodeService) GetNode(_ context.Context, req *connect.Request[nodev1.GetNodeRequest]) (*connect.Response[nodev1.GetNodeResponse], error) {
	if f.order != nil {
		*f.order = append(*f.order, "node")
	}
	resp, err := f.getNodeFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (f *fakeNodeService) AddNode(_ context.Context, _ *connect.Request[nodev1.AddNodeRequest]) (*connect.Response[nodev1.AddNodeResponse], error) {
	f.addNodeCalls++
	return connect.NewResponse(&nodev1.AddNodeResponse{}), nil
}

func startFakeServer(t *testing.T, svc *fakeNodeService) enableSSHDeps {
	t.Helper()
	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return enableSSHDeps{
		nodeClients: mockNodeClientFactory{serverURL: server.URL},
	}
}

type enableSSHOrder struct{ entries []string }

func (o *enableSSHOrder) add(entry string) { o.entries = append(o.entries, entry) }

type orderedPlatform struct{ order *enableSSHOrder }

func (p orderedPlatform) IsCompatible() bool {
	p.order.add("platform")
	return true
}

type orderedRegistrationStore struct {
	order  *enableSSHOrder
	reg    *register.DeviceRegistration
	exists bool
	err    error
}

func (s *orderedRegistrationStore) Save(*register.DeviceRegistration) error {
	return errors.New("Save must not be called")
}

func (s *orderedRegistrationStore) Load() (*register.DeviceRegistration, error) {
	return s.reg, s.err
}
func (s *orderedRegistrationStore) Delete() error { return errors.New("Delete must not be called") }
func (s *orderedRegistrationStore) Exists() (bool, error) {
	s.order.add("registration")
	return s.exists, s.err
}

type orderedEnableSSHStore struct {
	order *enableSSHOrder
	user  *entity.User
}

func (s orderedEnableSSHStore) GetCurrentUser() (*entity.User, error) {
	s.order.add("auth")
	return s.user, nil
}
func (orderedEnableSSHStore) GetAccessToken() (string, error) { return "token", nil }

type orderedTunnel struct {
	order *enableSSHOrder
	err   error
}

func (t orderedTunnel) EnsureConnected(context.Context) error {
	t.order.add("tunnel")
	return t.err
}

type reconnectingTunnel struct {
	order             *enableSSHOrder
	connected         *bool
	reconnectAttempts int
}

func (t *reconnectingTunnel) EnsureConnected(context.Context) error {
	t.order.add("tunnel")
	if !*t.connected {
		t.reconnectAttempts++
		*t.connected = true
	}
	return nil
}

type orderedProvisioner struct {
	order *enableSSHOrder
	err   error
}

func (p orderedProvisioner) Provision(
	context.Context,
	*terminal.Terminal,
	externalnode.TokenProvider,
	*register.DeviceRegistration,
	*entity.User,
	*nodev1.ExternalNode,
) error {
	p.order.add("provision")
	return p.err
}

type connectedTunnelProvisioner struct {
	order             *enableSSHOrder
	tunnelConnected   *bool
	observedConnected bool
}

func (p *connectedTunnelProvisioner) Provision(
	context.Context,
	*terminal.Terminal,
	externalnode.TokenProvider,
	*register.DeviceRegistration,
	*entity.User,
	*nodev1.ExternalNode,
) error {
	p.order.add("provision")
	p.observedConnected = *p.tunnelConnected
	if !p.observedConnected {
		return errors.New("SSH provisioning started before the Brev tunnel connected")
	}
	return nil
}

func newEnableSSHTestDeps(order *enableSSHOrder, factory externalnode.NodeClientFactory, registrationStore register.RegistrationStore, tunnelErr error) enableSSHDeps {
	return enableSSHDeps{
		platform:          orderedPlatform{order: order},
		nodeClients:       factory,
		registrationStore: registrationStore,
		tunnel:            orderedTunnel{order: order, err: tunnelErr},
		provisioner:       orderedProvisioner{order: order},
	}
}

func TestNewCmdEnableSSH_RejectsPositionalArguments(t *testing.T) {
	cmd := NewCmdEnableSSH(terminal.New(), &mockEnableSSHStore{})
	require.Error(t, cmd.Args(cmd, []string{"unexpected"}))
}

func TestRunEnableSSH_MissingRegistrationDirectsUserToJoin(t *testing.T) {
	order := &enableSSHOrder{}
	registrationStore := &orderedRegistrationStore{order: order, exists: false}
	deps := newEnableSSHTestDeps(order, nil, registrationStore, nil)

	err := runEnableSSH(context.Background(), terminal.New(), orderedEnableSSHStore{order: order}, deps)

	require.EqualError(t, err, `This machine has not joined a Brev network; run "brev join" first.`)
	require.Equal(t, []string{"platform", "registration"}, order.entries)
}

func TestRunEnableSSH_MissingBackendNodeDoesNotConnectOrProvision(t *testing.T) {
	order := &enableSSHOrder{}
	svc := &fakeNodeService{
		order: &order.entries,
		getNodeFn: func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{}, nil
		},
	}
	deps := startFakeServer(t, svc)
	registrationStore := &orderedRegistrationStore{order: order, exists: true, reg: &register.DeviceRegistration{ExternalNodeID: "unode_123", OrgID: "org_456"}}
	deps.platform = orderedPlatform{order: order}
	deps.registrationStore = registrationStore
	deps.tunnel = orderedTunnel{order: order}
	deps.provisioner = orderedProvisioner{order: order}

	err := runEnableSSH(context.Background(), terminal.New(), orderedEnableSSHStore{order: order, user: &entity.User{ID: "user_123"}}, deps)

	require.ErrorContains(t, err, "registered node was not returned by Brev")
	require.Equal(t, []string{"platform", "registration", "auth", "node"}, order.entries)
}

func TestRunEnableSSH_ConnectedTunnelProvisionsSSH(t *testing.T) {
	order := &enableSSHOrder{}
	svc := &fakeNodeService{
		order: &order.entries,
		getNodeFn: func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{ExternalNodeId: "unode_123"}}, nil
		},
	}
	deps := startFakeServer(t, svc)
	registrationStore := &orderedRegistrationStore{order: order, exists: true, reg: &register.DeviceRegistration{ExternalNodeID: "unode_123", OrgID: "org_456", DisplayName: "joined-node"}}
	deps.platform = orderedPlatform{order: order}
	deps.registrationStore = registrationStore
	deps.tunnel = orderedTunnel{order: order}
	deps.provisioner = orderedProvisioner{order: order}

	err := runEnableSSH(context.Background(), terminal.New(), orderedEnableSSHStore{order: order, user: &entity.User{ID: "user_123"}}, deps)

	require.NoError(t, err)
	require.Equal(t, []string{"platform", "registration", "auth", "node", "tunnel", "provision"}, order.entries)
}

func TestRunEnableSSH_ReconnectsBeforeProvisioning(t *testing.T) {
	order := &enableSSHOrder{}
	tunnelConnected := false
	svc := &fakeNodeService{
		order: &order.entries,
		getNodeFn: func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{ExternalNodeId: "unode_123"}}, nil
		},
	}
	deps := startFakeServer(t, svc)
	deps.platform = orderedPlatform{order: order}
	deps.registrationStore = &orderedRegistrationStore{order: order, exists: true, reg: &register.DeviceRegistration{ExternalNodeID: "unode_123", OrgID: "org_456"}}
	tunnel := &reconnectingTunnel{order: order, connected: &tunnelConnected}
	deps.tunnel = tunnel
	provisioner := &connectedTunnelProvisioner{order: order, tunnelConnected: &tunnelConnected}
	deps.provisioner = provisioner

	err := runEnableSSH(context.Background(), terminal.New(), orderedEnableSSHStore{order: order, user: &entity.User{ID: "user_123"}}, deps)

	require.NoError(t, err)
	require.Equal(t, 1, tunnel.reconnectAttempts)
	require.True(t, provisioner.observedConnected)
	require.Equal(t, []string{"platform", "registration", "auth", "node", "tunnel", "provision"}, order.entries)
}

func TestRunEnableSSH_TunnelFailureDoesNotProvision(t *testing.T) {
	tests := []struct {
		name       string
		tunnelErr  error
		wantErrMsg string
	}{
		{
			name:       "generic reconnect failure",
			tunnelErr:  errors.New("tunnel failed"),
			wantErrMsg: "enable SSH requires a connected Brev tunnel",
		},
		{
			name:       "connection remains unconfirmed",
			tunnelErr:  errors.New("Brev tunnel connection was not confirmed"),
			wantErrMsg: "Brev tunnel connection was not confirmed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &enableSSHOrder{}
			svc := &fakeNodeService{
				order: &order.entries,
				getNodeFn: func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
					return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{ExternalNodeId: "unode_123"}}, nil
				},
			}
			deps := startFakeServer(t, svc)
			deps.platform = orderedPlatform{order: order}
			deps.registrationStore = &orderedRegistrationStore{order: order, exists: true, reg: &register.DeviceRegistration{ExternalNodeID: "unode_123", OrgID: "org_456"}}
			deps.tunnel = orderedTunnel{order: order, err: tt.tunnelErr}
			deps.provisioner = orderedProvisioner{order: order}

			err := runEnableSSH(context.Background(), terminal.New(), orderedEnableSSHStore{order: order, user: &entity.User{ID: "user_123"}}, deps)

			require.ErrorContains(t, err, tt.wantErrMsg)
			require.NotContains(t, order.entries, "provision")
		})
	}
}

func TestRunEnableSSH_NeverAddsNode(t *testing.T) {
	order := &enableSSHOrder{}
	svc := &fakeNodeService{
		order: &order.entries,
		getNodeFn: func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{ExternalNodeId: "unode_123"}}, nil
		},
	}
	deps := startFakeServer(t, svc)
	deps.platform = orderedPlatform{order: order}
	deps.registrationStore = &orderedRegistrationStore{order: order, exists: true, reg: &register.DeviceRegistration{ExternalNodeID: "unode_123", OrgID: "org_456"}}
	deps.tunnel = orderedTunnel{order: order}
	deps.provisioner = orderedProvisioner{order: order}

	err := runEnableSSH(context.Background(), terminal.New(), orderedEnableSSHStore{order: order, user: &entity.User{ID: "user_123"}}, deps)

	require.NoError(t, err)
	require.Zero(t, svc.addNodeCalls)
}
