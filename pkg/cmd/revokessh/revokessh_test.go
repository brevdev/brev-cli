package revokessh

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
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

// --- mock types ---

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

type mockRevokeSSHStore struct {
	token string
}

func (m *mockRevokeSSHStore) GetAccessToken() (string, error) { return m.token, nil }

// fakeNodeService implements the server side of ExternalNodeService for testing.
type fakeNodeService struct {
	nodev1connect.UnimplementedExternalNodeServiceHandler
	revokeSSHFn func(*nodev1.RevokeNodeSSHAccessRequest) (*nodev1.RevokeNodeSSHAccessResponse, error)
}

func (f *fakeNodeService) RevokeNodeSSHAccess(_ context.Context, req *connect.Request[nodev1.RevokeNodeSSHAccessRequest]) (*connect.Response[nodev1.RevokeNodeSSHAccessResponse], error) {
	resp, err := f.revokeSSHFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- helpers ---

func tempUser(t *testing.T) *user.User {
	t.Helper()
	return &user.User{HomeDir: t.TempDir(), Username: "testuser"}
}

func seedAuthorizedKeys(t *testing.T, u *user.User, content string) {
	t.Helper()
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readAuthorizedKeys(t *testing.T, u *user.User) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(u.HomeDir, ".ssh", "authorized_keys"))
	if err != nil {
		t.Fatalf("reading authorized_keys: %v", err)
	}
	return string(data)
}

func baseDeps(t *testing.T, svc *fakeNodeService, regStore register.RegistrationStore, osUser *user.User) (revokeSSHDeps, *httptest.Server) {
	t.Helper()

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)

	return revokeSSHDeps{
		platform: mockPlatform{compatible: true},
		prompter: mockSelector{fn: func(_ string, items []string) string {
			if len(items) > 0 {
				return items[0]
			}
			return ""
		}},
		nodeClients:       mockNodeClientFactory{serverURL: server.URL},
		registrationStore: regStore,
		currentUser:       func() (*user.User, error) { return osUser, nil },
		listBrevKeys:      register.ListBrevAuthorizedKeys,
		removeKeyLine:     register.RemoveAuthorizedKeyLine,
	}, server
}

// --- tests ---

func Test_runRevokeSSH_NotCompatible(t *testing.T) {
	osUser := tempUser(t)
	regStore := &mockRegistrationStore{}
	store := &mockRevokeSSHStore{token: "tok"}
	svc := &fakeNodeService{}

	deps, server := baseDeps(t, svc, regStore, osUser)
	defer server.Close()
	deps.platform = mockPlatform{compatible: false}

	term := terminal.New()
	err := runRevokeSSH(context.Background(), term, store, deps)
	if err == nil {
		t.Fatal("expected error for incompatible platform")
	}
}

func Test_runRevokeSSH_NotRegistered(t *testing.T) {
	osUser := tempUser(t)
	regStore := &mockRegistrationStore{} // no registration
	store := &mockRevokeSSHStore{token: "tok"}
	svc := &fakeNodeService{}

	deps, server := baseDeps(t, svc, regStore, osUser)
	defer server.Close()

	term := terminal.New()
	err := runRevokeSSH(context.Background(), term, store, deps)
	if err == nil {
		t.Fatal("expected error when not registered")
	}
}

func Test_runRevokeSSH_NoBrevKeys(t *testing.T) {
	osUser := tempUser(t)
	// Seed only non-brev keys.
	seedAuthorizedKeys(t, osUser, "ssh-rsa EXISTING user@host\n")

	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
		},
	}
	store := &mockRevokeSSHStore{token: "tok"}
	svc := &fakeNodeService{}

	deps, server := baseDeps(t, svc, regStore, osUser)
	defer server.Close()

	term := terminal.New()
	err := runRevokeSSH(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("expected nil error for no brev keys, got: %v", err)
	}
}

func Test_runRevokeSSH_RevokeKeyWithUserID(t *testing.T) {
	osUser := tempUser(t)

	keyLine := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAliceKey # brev-cli user_id=user_2"
	seedAuthorizedKeys(t, osUser, strings.Join([]string{
		"ssh-rsa EXISTING user@host",
		keyLine,
		"",
	}, "\n"))

	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
		},
	}
	store := &mockRevokeSSHStore{token: "tok"}

	var gotReq *nodev1.RevokeNodeSSHAccessRequest
	svc := &fakeNodeService{
		revokeSSHFn: func(req *nodev1.RevokeNodeSSHAccessRequest) (*nodev1.RevokeNodeSSHAccessResponse, error) {
			gotReq = req
			return &nodev1.RevokeNodeSSHAccessResponse{}, nil
		},
	}

	deps, server := baseDeps(t, svc, regStore, osUser)
	defer server.Close()

	term := terminal.New()
	err := runRevokeSSH(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("runRevokeSSH failed: %v", err)
	}

	// Verify key was removed from file.
	remaining := readAuthorizedKeys(t, osUser)
	if strings.Contains(remaining, "AliceKey") {
		t.Errorf("expected key to be removed, still present:\n%s", remaining)
	}
	if !strings.Contains(remaining, "ssh-rsa EXISTING user@host") {
		t.Errorf("non-brev key was removed:\n%s", remaining)
	}

	// Verify RPC was called with correct params.
	if gotReq == nil {
		t.Fatal("expected RevokeNodeSSHAccess to be called")
	}
	if gotReq.GetExternalNodeId() != "unode_abc" {
		t.Errorf("expected node ID unode_abc, got %s", gotReq.GetExternalNodeId())
	}
	if gotReq.GetUserId() != "user_2" {
		t.Errorf("expected user ID user_2, got %s", gotReq.GetUserId())
	}
	if gotReq.GetLinuxUser() != "testuser" {
		t.Errorf("expected linux user testuser, got %s", gotReq.GetLinuxUser())
	}
}

func Test_runRevokeSSH_RevokeOldFormatKey_SkipsRPC(t *testing.T) {
	osUser := tempUser(t)

	// Old-format key: no user_id in the tag.
	keyLine := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBobKey # brev-cli"
	seedAuthorizedKeys(t, osUser, strings.Join([]string{
		keyLine,
		"",
	}, "\n"))

	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
		},
	}
	store := &mockRevokeSSHStore{token: "tok"}

	rpcCalled := false
	svc := &fakeNodeService{
		revokeSSHFn: func(_ *nodev1.RevokeNodeSSHAccessRequest) (*nodev1.RevokeNodeSSHAccessResponse, error) {
			rpcCalled = true
			return &nodev1.RevokeNodeSSHAccessResponse{}, nil
		},
	}

	deps, server := baseDeps(t, svc, regStore, osUser)
	defer server.Close()

	term := terminal.New()
	err := runRevokeSSH(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("runRevokeSSH failed: %v", err)
	}

	// Key should be removed from file.
	remaining := readAuthorizedKeys(t, osUser)
	if strings.Contains(remaining, "BobKey") {
		t.Errorf("expected key to be removed, still present:\n%s", remaining)
	}

	// RPC should NOT have been called.
	if rpcCalled {
		t.Error("expected RPC to be skipped for old-format key")
	}
}

func Test_runRevokeSSH_RPCFailure_StillRemovesKey(t *testing.T) {
	osUser := tempUser(t)

	keyLine := "ssh-ed25519 AAAA # brev-cli user_id=user_3"
	seedAuthorizedKeys(t, osUser, keyLine+"\n")

	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
		},
	}
	store := &mockRevokeSSHStore{token: "tok"}

	svc := &fakeNodeService{
		revokeSSHFn: func(_ *nodev1.RevokeNodeSSHAccessRequest) (*nodev1.RevokeNodeSSHAccessResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, nil)
		},
	}

	deps, server := baseDeps(t, svc, regStore, osUser)
	defer server.Close()

	term := terminal.New()
	// The command should succeed (key is removed) even though the RPC fails.
	err := runRevokeSSH(context.Background(), term, store, deps)
	if err != nil {
		t.Fatalf("expected no error (RPC failure is a warning), got: %v", err)
	}

	remaining := readAuthorizedKeys(t, osUser)
	if strings.Contains(remaining, "brev-cli") {
		t.Errorf("key should be removed even when RPC fails:\n%s", remaining)
	}
}

func Test_truncateKey(t *testing.T) {
	short := "ssh-rsa AAAA"
	if got := truncateKey(short, 60); got != short {
		t.Errorf("expected %q, got %q", short, got)
	}

	long := strings.Repeat("x", 100)
	got := truncateKey(long, 60)
	if len(got) != 63 { // 60 + "..."
		t.Errorf("expected length 63, got %d: %q", len(got), got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected suffix '...', got %q", got)
	}
}
