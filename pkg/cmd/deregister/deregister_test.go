package deregister

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os/user"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type mockDeregisterStore struct {
	user  *entity.User
	home  string
	token string
	err   error
}

func (m *mockDeregisterStore) GetCurrentUser() (*entity.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

func (m *mockDeregisterStore) GetBrevHomePath() (string, error) { return m.home, nil }
func (m *mockDeregisterStore) GetAccessToken() (string, error)  { return m.token, nil }

// fakeNodeService implements the server side of ExternalNodeService for testing.
type fakeNodeService struct {
	nodev1connect.UnimplementedExternalNodeServiceHandler
	removeNodeFn func(*nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error)
}

func (f *fakeNodeService) RemoveNode(_ context.Context, req *connect.Request[nodev1.RemoveNodeRequest]) (*connect.Response[nodev1.RemoveNodeResponse], error) {
	resp, err := f.removeNodeFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// mockRegistrationStore satisfies register.RegistrationStore for deregister tests.
type mockRegistrationStore struct {
	reg *register.DeviceRegistration
}

func (m *mockRegistrationStore) Save(reg *register.DeviceRegistration) error {
	m.reg = reg
	return nil
}

func (m *mockRegistrationStore) Load() (*register.DeviceRegistration, error) {
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

// mock types for deregisterDeps interfaces

type mockPlatform struct{ compatible bool }

func (m mockPlatform) IsCompatible() bool { return m.compatible }

type mockSelector struct {
	fn func(label string, items []string) string
}

func (m mockSelector) Select(label string, items []string) string {
	return m.fn(label, items)
}

type mockNetBirdUninstaller struct {
	called bool
	err    error
}

func (m *mockNetBirdUninstaller) Uninstall() error {
	m.called = true
	return m.err
}

type mockNodeClientFactory struct {
	serverURL string
}

func (m mockNodeClientFactory) NewNodeClient(provider register.TokenProvider, _ string) nodev1connect.ExternalNodeServiceClient {
	return register.NewNodeServiceClient(provider, m.serverURL)
}

type mockSSHKeyRemover struct {
	called bool
	err    error
}

func (m *mockSSHKeyRemover) RemoveBrevKeys(_ *user.User) error {
	m.called = true
	return m.err
}

// testDeregisterDeps returns deps with all side-effects stubbed. The
// prompter defaults to confirming all prompts.
func testDeregisterDeps(t *testing.T, svc *fakeNodeService, regStore register.RegistrationStore) (deregisterDeps, *httptest.Server) {
	t.Helper()

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)

	return deregisterDeps{
		platform: mockPlatform{compatible: true},
		prompter: mockSelector{fn: func(_ string, items []string) string {
			// Default: pick first item (Yes, ...)
			if len(items) > 0 {
				return items[0]
			}
			return ""
		}},
		netbird:           &mockNetBirdUninstaller{},
		nodeClients:       mockNodeClientFactory{serverURL: server.URL},
		registrationStore: regStore,
		sshKeys:           &mockSSHKeyRemover{},
	}, server
}

func Test_runDeregister_HappyPath(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
			DeviceID:       "dev-uuid",
		},
	}

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	var gotNodeID string
	svc := &fakeNodeService{
		removeNodeFn: func(req *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			gotNodeID = req.GetExternalNodeId()
			return &nodev1.RemoveNodeResponse{}, nil
		},
	}

	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("runDeregister failed: %v", err)
	}

	if gotNodeID != "unode_abc" {
		t.Errorf("expected node ID unode_abc, got %s", gotNodeID)
	}

	// Registration should be deleted
	exists, err := regStore.Exists()
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if exists {
		t.Error("expected registration to be deleted after deregister")
	}
}

func Test_runDeregister_UserCancels(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
		},
	}

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()

	deps.prompter = mockSelector{fn: func(_ string, _ []string) string {
		return "No, cancel"
	}}

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("expected nil error on cancel, got: %v", err)
	}

	// Registration should still exist
	exists, err := regStore.Exists()
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Error("registration should still exist after cancel")
	}
}

func Test_runDeregister_NotRegistered(t *testing.T) {
	regStore := &mockRegistrationStore{}

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err == nil {
		t.Fatal("expected error when not registered")
	}
}

func Test_runDeregister_RemoveNodeFails(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
		},
	}

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, nil)
		},
	}

	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err == nil {
		t.Fatal("expected error when RemoveNode fails")
	}

	// Registration should still exist (server-side removal failed)
	exists, err := regStore.Exists()
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Error("registration should still exist when RemoveNode fails")
	}
}

func Test_runDeregister_SkipsNetbirdUninstall(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
		},
	}

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return &nodev1.RemoveNodeResponse{}, nil
		},
	}

	netbird := &mockNetBirdUninstaller{}
	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()

	deps.prompter = mockSelector{fn: func(label string, _ []string) string {
		if label == "Would you also like to uninstall NetBird?" {
			return "No, keep NetBird installed"
		}
		return "Yes, proceed"
	}}
	deps.netbird = netbird

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("runDeregister failed: %v", err)
	}

	if netbird.called {
		t.Error("NetBird uninstall should not be called when user declines")
	}
}

func Test_runDeregister_CallsRemoveBrevKeys(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
		},
	}

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return &nodev1.RemoveNodeResponse{}, nil
		},
	}

	sshKeys := &mockSSHKeyRemover{}
	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()
	deps.sshKeys = sshKeys

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("runDeregister failed: %v", err)
	}

	if !sshKeys.called {
		t.Error("expected removeBrevKeys to be called during deregistration")
	}
}

func Test_runDeregister_RemoveBrevKeysFailureIsNonFatal(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
		},
	}

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return &nodev1.RemoveNodeResponse{}, nil
		},
	}

	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()
	deps.sshKeys = &mockSSHKeyRemover{err: fmt.Errorf("permission denied")}

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("expected deregister to succeed despite removeBrevKeys failure, got: %v", err)
	}

	// Registration should still be cleaned up.
	exists, err := regStore.Exists()
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if exists {
		t.Error("expected registration to be deleted even when SSH key cleanup fails")
	}
}
