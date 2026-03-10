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
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type mockDeregisterStore struct {
	user  *entity.User
	token string
	err   error
}

func (m *mockDeregisterStore) GetCurrentUser() (*entity.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

func (m *mockDeregisterStore) GetAccessToken() (string, error) { return m.token, nil }

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

type mockConfirmer struct{ confirm bool }

func (m mockConfirmer) ConfirmYesNo(_ string) bool { return m.confirm }

type mockNetBirdManager struct {
	called bool
	err    error
}

func (m *mockNetBirdManager) Install() error       { return m.err }
func (m *mockNetBirdManager) Uninstall() error     { m.called = true; return m.err }
func (m *mockNetBirdManager) EnsureRunning() error { return m.err }

type mockNodeClientFactory struct {
	serverURL string
}

func (m mockNodeClientFactory) NewNodeClient(provider externalnode.TokenProvider, _ string) nodev1connect.ExternalNodeServiceClient {
	return register.NewNodeServiceClient(provider, m.serverURL)
}

type mockSSHKeyRemover struct {
	called  bool
	err     error
	removed []string
}

func (m *mockSSHKeyRemover) RemoveBrevKeys(_ *user.User) ([]string, error) {
	m.called = true
	return m.removed, m.err
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
		confirmer:         mockConfirmer{confirm: true},
		netbird:           &mockNetBirdManager{},
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
		user: &entity.User{ID: "user_1"},

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
	err := runDeregister(context.Background(), term, store, deps, false)
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
		user: &entity.User{ID: "user_1"},

		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()

	deps.prompter = mockSelector{fn: func(_ string, _ []string) string {
		return "No, cancel"
	}}

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps, false)
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
		user: &entity.User{ID: "user_1"},

		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps, false)
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
		user: &entity.User{ID: "user_1"},

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
	err := runDeregister(context.Background(), term, store, deps, false)
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

func Test_runDeregister_AlwaysUninstallsNetbird(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
		},
	}

	store := &mockDeregisterStore{
		user: &entity.User{ID: "user_1"},

		token: "tok",
	}

	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return &nodev1.RemoveNodeResponse{}, nil
		},
	}

	netbird := &mockNetBirdManager{}
	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()
	deps.netbird = netbird

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps, false)
	if err != nil {
		t.Fatalf("runDeregister failed: %v", err)
	}

	if !netbird.called {
		t.Error("expected Brev tunnel uninstall to always be called during deregistration")
	}
}

func Test_runDeregister_RemoveBrevKeysHandling(t *testing.T) {
	tests := []struct {
		name       string
		sshKeys    *mockSSHKeyRemover
		wantCalled bool
	}{
		{"CallsRemoveBrevKeys", &mockSSHKeyRemover{}, true},
		{"FailureIsNonFatal", &mockSSHKeyRemover{err: fmt.Errorf("permission denied")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regStore := &mockRegistrationStore{
				reg: &register.DeviceRegistration{
					ExternalNodeID: "unode_abc",
					DisplayName:    "My Spark",
					OrgID:          "org_123",
				},
			}

			store := &mockDeregisterStore{
				user: &entity.User{ID: "user_1"},

				token: "tok",
			}

			svc := &fakeNodeService{
				removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
					return &nodev1.RemoveNodeResponse{}, nil
				},
			}

			deps, server := testDeregisterDeps(t, svc, regStore)
			defer server.Close()
			deps.sshKeys = tt.sshKeys

			term := terminal.New()
			err := runDeregister(context.Background(), term, store, deps, false)
			if err != nil {
				t.Fatalf("runDeregister failed: %v", err)
			}

			if tt.sshKeys.called != tt.wantCalled {
				t.Errorf("removeBrevKeys called = %v, want %v", tt.sshKeys.called, tt.wantCalled)
			}

			// Registration should be cleaned up regardless of SSH key result.
			exists, err := regStore.Exists()
			if err != nil {
				t.Fatalf("Exists error: %v", err)
			}
			if exists {
				t.Error("expected registration to be deleted")
			}
		})
	}
}
