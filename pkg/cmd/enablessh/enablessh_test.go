package enablessh

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"strings"
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

// tempUser returns a *user.User whose HomeDir points to a temporary directory.
func tempUser(t *testing.T) *user.User {
	t.Helper()
	return &user.User{HomeDir: t.TempDir()}
}

// readAuthorizedKeys is a test helper that reads ~/.ssh/authorized_keys.
func readAuthorizedKeys(t *testing.T, u *user.User) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(u.HomeDir, ".ssh", "authorized_keys"))
	if err != nil {
		t.Fatalf("reading authorized_keys: %v", err)
	}
	return string(data)
}

// --- RemoveBrevAuthorizedKeys ---

func Test_RemoveBrevAuthorizedKeys_RemovesTaggedKeys(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	content := strings.Join([]string{
		"ssh-rsa EXISTING user@host",
		"ssh-rsa BREVKEY1 " + register.DevplaneAuthorizedKeysComment("p1", "u1"),
		"ssh-ed25519 OTHERKEY admin@server",
		"ssh-rsa BREVKEY2 " + register.DevplaneAuthorizedKeysComment("p2", "u2"),
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	removed, err := register.RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("RemoveBrevAuthorizedKeys: %v", err)
	}

	if len(removed) != 2 {
		t.Errorf("expected 2 removed keys, got %d: %v", len(removed), removed)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, "#brev-portID:") {
		t.Errorf("brev keys still present:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa EXISTING user@host") {
		t.Errorf("non-brev key was removed:\n%s", result)
	}
	if !strings.Contains(result, "ssh-ed25519 OTHERKEY admin@server") {
		t.Errorf("non-brev key was removed:\n%s", result)
	}
}

func Test_RemoveBrevAuthorizedKeys_NoopWhenFileDoesNotExist(t *testing.T) {
	u := tempUser(t)

	removed, err := register.RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected no removed keys, got %v", removed)
	}
}

func Test_RemoveBrevAuthorizedKeys_NoopWhenNoBrevKeys(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	original := "ssh-rsa EXISTING user@host\nssh-ed25519 OTHER admin@server\n"
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	removed, err := register.RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("RemoveBrevAuthorizedKeys: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected no removed keys, got %v", removed)
	}

	result := readAuthorizedKeys(t, u)
	if result != original {
		t.Errorf("file was modified when it shouldn't have been.\nwant:\n%s\ngot:\n%s", original, result)
	}
}

// --- RemoveAuthorizedKey (specific key removal) ---

func Test_RemoveAuthorizedKey_RemovesOnlyTargetKey(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	content := strings.Join([]string{
		"ssh-rsa KEEP1 user@host",
		"ssh-rsa TARGET " + register.DevplaneAuthorizedKeysComment("p1", "u1"),
		"ssh-rsa KEEP2 admin@server",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := register.RemoveAuthorizedKey(u, "ssh-rsa TARGET"); err != nil {
		t.Fatalf("RemoveAuthorizedKey: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, "TARGET") {
		t.Errorf("target key still present:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa KEEP1 user@host") {
		t.Errorf("unrelated key was removed:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa KEEP2 admin@server") {
		t.Errorf("unrelated key was removed:\n%s", result)
	}
}

func Test_RemoveAuthorizedKey_NoopWhenKeyNotPresent(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	original := "ssh-rsa EXISTING user@host\n"
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := register.RemoveAuthorizedKey(u, "ssh-rsa NOTHERE"); err != nil {
		t.Fatalf("RemoveAuthorizedKey: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if !strings.Contains(result, "ssh-rsa EXISTING user@host") {
		t.Errorf("existing key was removed:\n%s", result)
	}
}

func Test_RemoveAuthorizedKey_NoopCases(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"MissingFile", "ssh-rsa SOMEKEY"},
		{"EmptyKey", ""},
		{"WhitespaceKey", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := tempUser(t)
			if err := register.RemoveAuthorizedKey(u, tt.key); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

func Test_RemoveAuthorizedKey_DoesNotRemoveOtherBrevKeys(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	content := strings.Join([]string{
		"ssh-rsa ALICE_KEY " + register.DevplaneAuthorizedKeysComment("p1", "u1"),
		"ssh-rsa BOB_KEY " + register.DevplaneAuthorizedKeysComment("p2", "u2"),
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// Remove only Alice's key — Bob's should stay.
	if err := register.RemoveAuthorizedKey(u, "ssh-rsa ALICE_KEY"); err != nil {
		t.Fatalf("RemoveAuthorizedKey: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, "ALICE_KEY") {
		t.Errorf("Alice's key still present:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa BOB_KEY") {
		t.Errorf("Bob's key was removed:\n%s", result)
	}
}

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
