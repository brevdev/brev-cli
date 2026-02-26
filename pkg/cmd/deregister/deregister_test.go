package deregister

import (
	"context"
	"net/http/httptest"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/afero"
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

func setupDeregisterTestFs(t *testing.T) (string, func()) {
	t.Helper()
	origFs := files.AppFs
	files.AppFs = afero.NewMemMapFs()
	brevHome := "/home/testuser/.brev"
	if err := files.AppFs.MkdirAll(brevHome, 0o700); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	return brevHome, func() { files.AppFs = origFs }
}

// testDeregisterDeps returns deps with all side-effects stubbed. The
// promptSelect defaults to confirming all prompts.
func testDeregisterDeps(t *testing.T, svc *fakeNodeService) (deregisterDeps, *httptest.Server) {
	t.Helper()

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)

	return deregisterDeps{
		goos: "linux",
		promptSelect: func(_ string, items []string) string {
			// Default: pick first item (Yes, ...)
			if len(items) > 0 {
				return items[0]
			}
			return ""
		},
		uninstallNetbird: func(_ *terminal.Terminal) error { return nil },
		newNodeClient: func(provider register.TokenProvider, _ string) nodev1connect.ExternalNodeServiceClient {
			return register.NewNodeServiceClient(provider, server.URL)
		},
		registrationExists: register.RegistrationExists,
		loadRegistration:   register.LoadRegistration,
		deleteRegistration: register.DeleteRegistration,
	}, server
}

func Test_runDeregister_HappyPath(t *testing.T) {
	brevHome, cleanup := setupDeregisterTestFs(t)
	defer cleanup()

	// Pre-save a registration
	_ = register.SaveRegistration(brevHome, &register.DeviceRegistration{
		ExternalNodeID: "unode_abc",
		DisplayName:    "My Spark",
		OrgID:          "org_123",
		DeviceID:       "dev-uuid",
	})

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  brevHome,
		token: "tok",
	}

	var gotNodeID, gotOrgID string
	svc := &fakeNodeService{
		removeNodeFn: func(req *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			gotNodeID = req.GetExternalNodeId()
			gotOrgID = req.GetOrganizationId()
			return &nodev1.RemoveNodeResponse{}, nil
		},
	}

	deps, server := testDeregisterDeps(t, svc)
	defer server.Close()

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("runDeregister failed: %v", err)
	}

	if gotNodeID != "unode_abc" {
		t.Errorf("expected node ID unode_abc, got %s", gotNodeID)
	}
	if gotOrgID != "org_123" {
		t.Errorf("expected org ID org_123, got %s", gotOrgID)
	}

	// Registration file should be deleted
	exists, err := register.RegistrationExists(brevHome)
	if err != nil {
		t.Fatalf("RegistrationExists error: %v", err)
	}
	if exists {
		t.Error("expected registration file to be deleted after deregister")
	}
}

func Test_runDeregister_UserCancels(t *testing.T) {
	brevHome, cleanup := setupDeregisterTestFs(t)
	defer cleanup()

	_ = register.SaveRegistration(brevHome, &register.DeviceRegistration{
		ExternalNodeID: "unode_abc",
		DisplayName:    "My Spark",
		OrgID:          "org_123",
	})

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  brevHome,
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testDeregisterDeps(t, svc)
	defer server.Close()

	callCount := 0
	deps.promptSelect = func(_ string, _ []string) string {
		callCount++
		if callCount == 2 {
			// Second prompt is the confirmation — cancel it
			return "No, cancel"
		}
		return "No, keep NetBird installed"
	}

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("expected nil error on cancel, got: %v", err)
	}

	// Registration file should still exist
	exists, err := register.RegistrationExists(brevHome)
	if err != nil {
		t.Fatalf("RegistrationExists error: %v", err)
	}
	if !exists {
		t.Error("registration should still exist after cancel")
	}
}

func Test_runDeregister_NotRegistered(t *testing.T) {
	brevHome, cleanup := setupDeregisterTestFs(t)
	defer cleanup()

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  brevHome,
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testDeregisterDeps(t, svc)
	defer server.Close()

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err == nil {
		t.Fatal("expected error when not registered")
	}
}

func Test_runDeregister_RemoveNodeFails(t *testing.T) {
	brevHome, cleanup := setupDeregisterTestFs(t)
	defer cleanup()

	_ = register.SaveRegistration(brevHome, &register.DeviceRegistration{
		ExternalNodeID: "unode_abc",
		DisplayName:    "My Spark",
		OrgID:          "org_123",
	})

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  brevHome,
		token: "tok",
	}

	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, nil)
		},
	}

	deps, server := testDeregisterDeps(t, svc)
	defer server.Close()

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err == nil {
		t.Fatal("expected error when RemoveNode fails")
	}

	// Registration file should still exist (server-side removal failed)
	exists, err := register.RegistrationExists(brevHome)
	if err != nil {
		t.Fatalf("RegistrationExists error: %v", err)
	}
	if !exists {
		t.Error("registration should still exist when RemoveNode fails")
	}
}

func Test_runDeregister_SkipsNetbirdUninstall(t *testing.T) {
	brevHome, cleanup := setupDeregisterTestFs(t)
	defer cleanup()

	_ = register.SaveRegistration(brevHome, &register.DeviceRegistration{
		ExternalNodeID: "unode_abc",
		DisplayName:    "My Spark",
		OrgID:          "org_123",
	})

	store := &mockDeregisterStore{
		user:  &entity.User{ID: "user_1"},
		home:  brevHome,
		token: "tok",
	}

	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return &nodev1.RemoveNodeResponse{}, nil
		},
	}

	uninstallCalled := false
	deps, server := testDeregisterDeps(t, svc)
	defer server.Close()

	deps.promptSelect = func(label string, items []string) string {
		if label == "Would you also like to uninstall NetBird?" {
			return "No, keep NetBird installed"
		}
		return "Yes, proceed"
	}
	deps.uninstallNetbird = func(_ *terminal.Terminal) error {
		uninstallCalled = true
		return nil
	}

	term := terminal.New()
	err := runDeregister(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("runDeregister failed: %v", err)
	}

	if uninstallCalled {
		t.Error("NetBird uninstall should not be called when user declines")
	}
}
