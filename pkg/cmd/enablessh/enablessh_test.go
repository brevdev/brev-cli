package enablessh

import (
	"context"
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
)

// tempUser returns a *user.User whose HomeDir points to a temporary directory.
func tempUser(t *testing.T) *user.User {
	t.Helper()
	return &user.User{HomeDir: t.TempDir()}
}

// readAuthorizedKeys is a test helper that reads ~/.ssh/authorized_keys.
func readAuthorizedKeys(t *testing.T, u *user.User) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(u.HomeDir, ".ssh", "authorized_keys"))
	if err != nil {
		t.Fatalf("reading authorized_keys: %v", err)
	}
	return string(data)
}

// --- RemoveBrevAuthorizedKeys ---

func Test_RemoveBrevAuthorizedKeys_RemovesTaggedKeys(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	content := strings.Join([]string{
		"ssh-rsa EXISTING user@host",
		"ssh-rsa BREVKEY1 " + register.DevplaneAuthorizedKeysComment("p1", "u1"),
		"ssh-ed25519 OTHERKEY admin@server",
		"ssh-rsa BREVKEY2 " + register.DevplaneAuthorizedKeysComment("p2", "u2"),
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	removed, err := register.RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("RemoveBrevAuthorizedKeys: %v", err)
	}

	if len(removed) != 2 {
		t.Errorf("expected 2 removed keys, got %d: %v", len(removed), removed)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, "#brev-portID:") {
		t.Errorf("brev keys still present:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa EXISTING user@host") {
		t.Errorf("non-brev key was removed:\n%s", result)
	}
	if !strings.Contains(result, "ssh-ed25519 OTHERKEY admin@server") {
		t.Errorf("non-brev key was removed:\n%s", result)
	}
}

func Test_RemoveBrevAuthorizedKeys_NoopWhenFileDoesNotExist(t *testing.T) {
	u := tempUser(t)

	removed, err := register.RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected no removed keys, got %v", removed)
	}
}

func Test_RemoveBrevAuthorizedKeys_NoopWhenNoBrevKeys(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	original := "ssh-rsa EXISTING user@host\nssh-ed25519 OTHER admin@server\n"
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	removed, err := register.RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("RemoveBrevAuthorizedKeys: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected no removed keys, got %v", removed)
	}

	result := readAuthorizedKeys(t, u)
	if result != original {
		t.Errorf("file was modified when it shouldn't have been.\nwant:\n%s\ngot:\n%s", original, result)
	}
}

// --- RemoveAuthorizedKey (specific key removal) ---

func Test_RemoveAuthorizedKey_RemovesOnlyTargetKey(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	content := strings.Join([]string{
		"ssh-rsa KEEP1 user@host",
		"ssh-rsa TARGET " + register.DevplaneAuthorizedKeysComment("p1", "u1"),
		"ssh-rsa KEEP2 admin@server",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := register.RemoveAuthorizedKey(u, "ssh-rsa TARGET"); err != nil {
		t.Fatalf("RemoveAuthorizedKey: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, "TARGET") {
		t.Errorf("target key still present:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa KEEP1 user@host") {
		t.Errorf("unrelated key was removed:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa KEEP2 admin@server") {
		t.Errorf("unrelated key was removed:\n%s", result)
	}
}

func Test_RemoveAuthorizedKey_NoopWhenKeyNotPresent(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	original := "ssh-rsa EXISTING user@host\n"
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := register.RemoveAuthorizedKey(u, "ssh-rsa NOTHERE"); err != nil {
		t.Fatalf("RemoveAuthorizedKey: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if !strings.Contains(result, "ssh-rsa EXISTING user@host") {
		t.Errorf("existing key was removed:\n%s", result)
	}
}

func Test_RemoveAuthorizedKey_NoopCases(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"MissingFile", "ssh-rsa SOMEKEY"},
		{"EmptyKey", ""},
		{"WhitespaceKey", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := tempUser(t)
			if err := register.RemoveAuthorizedKey(u, tt.key); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

func Test_RemoveAuthorizedKey_DoesNotRemoveOtherBrevKeys(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	content := strings.Join([]string{
		"ssh-rsa ALICE_KEY " + register.DevplaneAuthorizedKeysComment("p1", "u1"),
		"ssh-rsa BOB_KEY " + register.DevplaneAuthorizedKeysComment("p2", "u2"),
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// Remove only Alice's key — Bob's should stay.
	if err := register.RemoveAuthorizedKey(u, "ssh-rsa ALICE_KEY"); err != nil {
		t.Fatalf("RemoveAuthorizedKey: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, "ALICE_KEY") {
		t.Errorf("Alice's key still present:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa BOB_KEY") {
		t.Errorf("Bob's key was removed:\n%s", result)
	}
}

type mockNodeClientFactory struct{ serverURL string }

func (m mockNodeClientFactory) NewNodeClient(provider externalnode.TokenProvider, _ string) nodev1connect.ExternalNodeServiceClient {
	return register.NewNodeServiceClient(provider, m.serverURL)
}

type mockEnableSSHStore struct {
	token string
}

func (m *mockEnableSSHStore) GetCurrentUser() (interface{}, error) { return nil, nil }
func (m *mockEnableSSHStore) GetAccessToken() (string, error)      { return m.token, nil }

// fakeNodeService implements the server side of ExternalNodeService for testing.
type fakeNodeService struct {
	nodev1connect.UnimplementedExternalNodeServiceHandler
	getNodeFn func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error)
}

func (f *fakeNodeService) GetNode(_ context.Context, req *connect.Request[nodev1.GetNodeRequest]) (*connect.Response[nodev1.GetNodeResponse], error) {
	resp, err := f.getNodeFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func startFakeServer(t *testing.T, svc *fakeNodeService) (enableSSHDeps, *httptest.Server) {
	t.Helper()
	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return enableSSHDeps{
		nodeClients: mockNodeClientFactory{serverURL: server.URL},
	}, server
}

func Test_fetchRegisteredNode(t *testing.T) {
	svc := &fakeNodeService{
		getNodeFn: func(req *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			if req.GetExternalNodeId() != "unode_abc" {
				t.Fatalf("unexpected node id %q", req.GetExternalNodeId())
			}
			return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{
				ExternalNodeId: "unode_abc",
				Ports:          []*nodev1.Port{{PortId: "port_1", PortNumber: 11640, ServerPort: 22}},
			}}, nil
		},
	}
	deps, _ := startFakeServer(t, svc)
	store := &mockEnableSSHStore{token: "tok"}
	reg := &register.DeviceRegistration{ExternalNodeID: "unode_abc", OrgID: "org_1"}

	node, err := fetchRegisteredNode(context.Background(), deps, store, reg)
	if err != nil {
		t.Fatal(err)
	}
	if len(node.GetPorts()) != 1 || node.GetPorts()[0].GetPortId() != "port_1" {
		t.Fatalf("unexpected node: %+v", node)
	}
}
