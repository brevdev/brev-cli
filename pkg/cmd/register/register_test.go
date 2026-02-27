package register

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

// mockRegisterStore satisfies RegisterStore for orchestration tests.
type mockRegisterStore struct {
	user  *entity.User
	org   *entity.Organization
	home  string
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

func (m *mockRegisterStore) GetBrevHomePath() (string, error) { return m.home, nil }
func (m *mockRegisterStore) GetAccessToken() (string, error)  { return m.token, nil }

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

// testRegisterDeps returns deps with all side effects stubbed out, and a fake
// ConnectRPC server backed by the provided fakeNodeService.
func testRegisterDeps(t *testing.T, svc *fakeNodeService, regStore RegistrationStore) (registerDeps, *httptest.Server) {
	t.Helper()

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)

	return registerDeps{
		goos:            "linux",
		promptYesNo:     func(_ string) bool { return true },
		installNetbird:  func() error { return nil },
		runSetupCommand: func(_ string) error { return nil },
		newNodeClient: func(provider TokenProvider, _ string) nodev1connect.ExternalNodeServiceClient {
			return NewNodeServiceClient(provider, server.URL)
		},
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
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	var gotSetupCmd string
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
				},
				SetupCommand: "netbird up --key abc",
			}, nil
		},
	}

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.runSetupCommand = func(cmd string) error {
		gotSetupCmd = cmd
		return nil
	}

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
	if gotSetupCmd != "netbird up --key abc" {
		t.Errorf("expected setup command 'netbird up --key abc', got %q", gotSetupCmd)
	}
}

func Test_runRegister_UserCancels(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.promptYesNo = func(_ string) bool { return false }

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
	regStore := &mockRegistrationStore{
		reg: &DeviceRegistration{
			ExternalNodeID: "unode_existing",
			DisplayName:    "Existing",
		},
	}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "My Spark", deps)
	if err == nil {
		t.Fatal("expected error for already-registered machine")
	}
}

func Test_runRegister_NoOrganization(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockRegisterStore{
		user:  &entity.User{ID: "user_1"},
		org:   nil,
		home:  "/home/testuser/.brev",
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
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		home:  "/home/testuser/.brev",
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
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	setupCalled := false
	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			return &nodev1.AddNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: "unode_abc",
					OrganizationId: "org_123",
					Name:           req.GetName(),
					DeviceId:       req.GetDeviceId(),
				},
				// No SetupCommand
			}, nil
		},
	}

	deps, server := testRegisterDeps(t, svc, regStore)
	defer server.Close()

	deps.runSetupCommand = func(_ string) error {
		setupCalled = true
		return nil
	}

	term := terminal.New()
	err := runRegister(context.Background(), term, store, "My Spark", deps)
	if err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	if setupCalled {
		t.Error("setup command should not be called when empty")
	}
}
