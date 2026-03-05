package grantssh

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

// mock types for grantSSHDeps interfaces

type mockPlatform struct{ compatible bool }

func (m mockPlatform) IsCompatible() bool { return m.compatible }

type mockSelector struct {
	fn func(label string, items []string) string
}

func (m mockSelector) Select(label string, items []string) string {
	return m.fn(label, items)
}

type mockNodeClientFactory struct {
	serverURL string
}

func (m mockNodeClientFactory) NewNodeClient(provider externalnode.TokenProvider, _ string) nodev1connect.ExternalNodeServiceClient {
	return register.NewNodeServiceClient(provider, m.serverURL)
}

// mockRegistrationStore satisfies register.RegistrationStore.
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

// mockGrantSSHStore satisfies GrantSSHStore.
type mockGrantSSHStore struct {
	user        *entity.User
	org         *entity.Organization
	home        string
	token       string
	attachments []entity.OrgRoleAttachment
	users       map[string]*entity.User
	err         error
}

func (m *mockGrantSSHStore) GetCurrentUser() (*entity.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

func (m *mockGrantSSHStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return m.org, nil
}

func (m *mockGrantSSHStore) GetBrevHomePath() (string, error) { return m.home, nil }
func (m *mockGrantSSHStore) GetAccessToken() (string, error)  { return m.token, nil }

func (m *mockGrantSSHStore) GetOrgRoleAttachments(_ string) ([]entity.OrgRoleAttachment, error) {
	return m.attachments, nil
}

func (m *mockGrantSSHStore) GetUserByID(userID string) (*entity.User, error) {
	u, ok := m.users[userID]
	if !ok {
		return nil, fmt.Errorf("user %s not found", userID)
	}
	return u, nil
}

// fakeNodeService implements the server side of ExternalNodeService for testing.
type fakeNodeService struct {
	nodev1connect.UnimplementedExternalNodeServiceHandler
	grantSSHFn func(*nodev1.GrantNodeSSHAccessRequest) (*nodev1.GrantNodeSSHAccessResponse, error)
}

func (f *fakeNodeService) GrantNodeSSHAccess(_ context.Context, req *connect.Request[nodev1.GrantNodeSSHAccessRequest]) (*connect.Response[nodev1.GrantNodeSSHAccessResponse], error) {
	resp, err := f.grantSSHFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// authorizedKeysPath returns the path to the current user's authorized_keys file.
func authorizedKeysPath(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}
	return filepath.Join(u.HomeDir, ".ssh", "authorized_keys")
}

// installTestAuthorizedKey writes a pubkey to the current user's
// ~/.ssh/authorized_keys so that checkSSHEnabled passes.
func installTestAuthorizedKey(t *testing.T, pubKey string) {
	t.Helper()
	authKeysPath := authorizedKeysPath(t)
	sshDir := filepath.Dir(authKeysPath)

	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}

	existing, _ := os.ReadFile(authKeysPath) // #nosec G304
	content := string(existing) + pubKey + "\n"
	if err := os.WriteFile(authKeysPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write authorized_keys: %v", err)
	}
}

// cleanupSSHKey removes the given pubkey from authorized_keys, restoring the
// file to its previous state.
func cleanupSSHKey(t *testing.T, pubKey string) {
	t.Helper()
	path := authorizedKeysPath(t)
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return // nothing to clean up
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if line != pubKey {
			kept = append(kept, line)
		}
	}
	os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o600) //nolint:errcheck // best-effort cleanup
}

func testGrantSSHDeps(t *testing.T, svc *fakeNodeService, regStore register.RegistrationStore) (grantSSHDeps, *httptest.Server) {
	t.Helper()

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)

	return grantSSHDeps{
		platform: mockPlatform{compatible: true},
		prompter: mockSelector{fn: func(_ string, items []string) string {
			if len(items) > 0 {
				return items[0]
			}
			return ""
		}},
		nodeClients:       mockNodeClientFactory{serverURL: server.URL},
		registrationStore: regStore,
	}, server
}

func Test_runGrantSSH_NotCompatible(t *testing.T) {
	regStore := &mockRegistrationStore{}
	store := &mockGrantSSHStore{
		user:  &entity.User{ID: "user_1"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testGrantSSHDeps(t, svc, regStore)
	defer server.Close()

	deps.platform = mockPlatform{compatible: false}

	term := terminal.New()
	err := runGrantSSH(context.Background(), term, store, deps)
	if err == nil {
		t.Fatal("expected error for incompatible platform")
	}
}

func Test_runGrantSSH_NotRegistered(t *testing.T) {
	regStore := &mockRegistrationStore{} // no registration

	store := &mockGrantSSHStore{
		user:  &entity.User{ID: "user_1"},
		home:  "/home/testuser/.brev",
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testGrantSSHDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runGrantSSH(context.Background(), term, store, deps)
	if err == nil {
		t.Fatal("expected error when not registered")
	}
}

func Test_runGrantSSH_HappyPath(t *testing.T) {
	pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey test@brev"
	installTestAuthorizedKey(t, pubKey)
	defer cleanupSSHKey(t, pubKey)

	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
			DeviceID:       "dev-uuid",
		},
	}

	targetUser := &entity.User{
		ID:        "user_2",
		Name:      "Alice",
		Email:     "alice@example.com",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAliceKey alice@brev",
	}

	store := &mockGrantSSHStore{
		user:  &entity.User{ID: "user_1", PublicKey: pubKey},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		home:  "/home/testuser/.brev",
		token: "tok",
		attachments: []entity.OrgRoleAttachment{
			{Subject: "user_1"}, // current user, should be filtered
			{Subject: "user_2"},
		},
		users: map[string]*entity.User{
			"user_2": targetUser,
		},
	}

	var gotReq *nodev1.GrantNodeSSHAccessRequest
	svc := &fakeNodeService{
		grantSSHFn: func(req *nodev1.GrantNodeSSHAccessRequest) (*nodev1.GrantNodeSSHAccessResponse, error) {
			gotReq = req
			return &nodev1.GrantNodeSSHAccessResponse{}, nil
		},
	}

	deps, server := testGrantSSHDeps(t, svc, regStore)
	defer server.Close()

	register.SetTestSSHPort(22)
	defer register.ClearTestSSHPort()

	term := terminal.New()
	err := runGrantSSH(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("runGrantSSH failed: %v", err)
	}

	if gotReq == nil {
		t.Fatal("expected GrantNodeSSHAccess to be called")
	}
	if gotReq.GetExternalNodeId() != "unode_abc" {
		t.Errorf("expected node ID unode_abc, got %s", gotReq.GetExternalNodeId())
	}
	if gotReq.GetUserId() != "user_2" {
		t.Errorf("expected user ID user_2, got %s", gotReq.GetUserId())
	}
}

func Test_runGrantSSH_RPCFailure(t *testing.T) {
	pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey2 test@brev"
	installTestAuthorizedKey(t, pubKey)
	defer cleanupSSHKey(t, pubKey)

	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
		},
	}

	store := &mockGrantSSHStore{
		user:  &entity.User{ID: "user_1", PublicKey: pubKey},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		home:  "/home/testuser/.brev",
		token: "tok",
		attachments: []entity.OrgRoleAttachment{
			{Subject: "user_2"},
		},
		users: map[string]*entity.User{
			"user_2": {ID: "user_2", Name: "Alice", Email: "alice@example.com"},
		},
	}

	svc := &fakeNodeService{
		grantSSHFn: func(_ *nodev1.GrantNodeSSHAccessRequest) (*nodev1.GrantNodeSSHAccessResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, nil)
		},
	}

	deps, server := testGrantSSHDeps(t, svc, regStore)
	defer server.Close()

	register.SetTestSSHPort(22)
	defer register.ClearTestSSHPort()

	term := terminal.New()
	err := runGrantSSH(context.Background(), term, store, deps)
	if err == nil {
		t.Fatal("expected error when GrantNodeSSHAccess fails")
	}
}

func Test_runGrantSSH_NoOtherMembers(t *testing.T) {
	pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey3 test@brev"
	installTestAuthorizedKey(t, pubKey)
	defer cleanupSSHKey(t, pubKey)

	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
		},
	}

	store := &mockGrantSSHStore{
		user:  &entity.User{ID: "user_1", PublicKey: pubKey},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		home:  "/home/testuser/.brev",
		token: "tok",
		attachments: []entity.OrgRoleAttachment{
			{Subject: "user_1"}, // only current user, no others
		},
		users: map[string]*entity.User{},
	}

	svc := &fakeNodeService{}
	deps, server := testGrantSSHDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	err := runGrantSSH(context.Background(), term, store, deps)
	if err == nil {
		t.Fatal("expected error when no other members exist")
	}
}
