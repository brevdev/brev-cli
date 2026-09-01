package enablessh

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

func tempUser(t *testing.T) *user.User {
	t.Helper()
	return &user.User{HomeDir: t.TempDir()}
}

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

func (m *mockEnableSSHStore) GetCurrentUser() (*entity.User, error) { return &entity.User{}, nil }
func (m *mockEnableSSHStore) GetAccessToken() (string, error)       { return m.token, nil }

type mockSelector struct {
	choice        string
	inputLineFunc func(*terminal.Terminal, string) (string, error)
}

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

func (m mockSelector) InputLine(t *terminal.Terminal, label string) (string, error) {
	if m.inputLineFunc != nil {
		return m.inputLineFunc(t, label)
	}
	return "", nil
}

type fakeNodeService struct {
	nodev1connect.UnimplementedExternalNodeServiceHandler
	getNodeFn  func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error)
	openPortFn func(*nodev1.OpenPortRequest) (*nodev1.OpenPortResponse, error)
	grantCalls int
	openCalls  int
}

func (f *fakeNodeService) GrantNodeSSHAccess(_ context.Context, _ *connect.Request[nodev1.GrantNodeSSHAccessRequest]) (*connect.Response[nodev1.GrantNodeSSHAccessResponse], error) {
	f.grantCalls++
	return connect.NewResponse(&nodev1.GrantNodeSSHAccessResponse{}), nil
}

func (f *fakeNodeService) OpenPort(_ context.Context, req *connect.Request[nodev1.OpenPortRequest]) (*connect.Response[nodev1.OpenPortResponse], error) {
	f.openCalls++
	if f.openPortFn != nil {
		resp, err := f.openPortFn(req.Msg)
		if err != nil {
			return nil, err
		}
		return connect.NewResponse(resp), nil
	}
	return connect.NewResponse(&nodev1.OpenPortResponse{
		Port: &nodev1.Port{
			PortId:     fmt.Sprintf("port_%d", req.Msg.GetPortNumber()),
			Protocol:   req.Msg.GetProtocol(),
			PortNumber: 41000,
			ServerPort: req.Msg.GetPortNumber(),
		},
	}), nil
}

func (f *fakeNodeService) GetNode(_ context.Context, req *connect.Request[nodev1.GetNodeRequest]) (*connect.Response[nodev1.GetNodeResponse], error) {
	resp, err := f.getNodeFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func startFakeServer(t *testing.T, svc *fakeNodeService) enableSSHDeps {
	t.Helper()
	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return enableSSHDeps{
		nodeClients: mockNodeClientFactory{serverURL: server.URL},
		prompter:    mockSelector{},
	}
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
	deps := startFakeServer(t, svc)
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

func Test_findExistingSSHPortMatchesDestinationPort(t *testing.T) {
	node := &nodev1.ExternalNode{Ports: []*nodev1.Port{
		{PortId: "port_other", PortNumber: 22, ServerPort: 8080},
		{PortId: "port_ssh", PortNumber: 11640, ServerPort: 22},
	}}

	got := findExistingSSHPort(node, 22)
	if got == nil || got.GetPortId() != "port_ssh" {
		t.Fatalf("expected destination port 22 mapping, got %+v", got)
	}
}

func Test_isPortAlreadyAllocatedError(t *testing.T) {
	err := fmt.Errorf("failed to allocate port: internal: 400 Bad Request: Port 22 is already allocated for this client")
	if !isPortAlreadyAllocatedError(err, 22) {
		t.Fatal("expected matching already-allocated error to be recognized")
	}
	if isPortAlreadyAllocatedError(err, 2222) {
		t.Fatal("must not recognize an allocation error for a different port")
	}
	if isPortAlreadyAllocatedError(fmt.Errorf("permission denied"), 22) {
		t.Fatal("must not recognize an unrelated error")
	}
}

func Test_ensureSSHPortPromptsThenReusesSelectedPort(t *testing.T) {
	svc := &fakeNodeService{
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{Ports: []*nodev1.Port{
				{PortId: "port_ssh", PortNumber: 11640, ServerPort: 22},
			}}}, nil
		},
	}
	deps := startFakeServer(t, svc)
	reg := &register.DeviceRegistration{ExternalNodeID: "unode_abc", OrgID: "org_1"}

	if err := ensureSSHPort(context.Background(), terminal.New(), deps, &mockEnableSSHStore{}, reg, enableSSHOpts{interactive: true}); err != nil {
		t.Fatalf("ensureSSHPort: %v", err)
	}
	if svc.openCalls != 0 {
		t.Fatalf("OpenPort called %d times, want 0", svc.openCalls)
	}
}

func Test_ensureSSHPortDoesNotAssumeExistingDifferentPort(t *testing.T) {
	svc := &fakeNodeService{
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{Ports: []*nodev1.Port{
				{PortId: "port_ssh", PortNumber: 11640, ServerPort: 22},
			}}}, nil
		},
	}
	deps := startFakeServer(t, svc)
	deps.prompter = mockSelector{inputLineFunc: func(*terminal.Terminal, string) (string, error) {
		return "2222", nil
	}}
	reg := &register.DeviceRegistration{ExternalNodeID: "unode_abc", OrgID: "org_1"}

	if err := ensureSSHPort(context.Background(), terminal.New(), deps, &mockEnableSSHStore{}, reg, enableSSHOpts{interactive: true}); err != nil {
		t.Fatalf("ensureSSHPort: %v", err)
	}
	if svc.openCalls != 1 {
		t.Fatalf("OpenPort called %d times, want 1", svc.openCalls)
	}
}

func Test_ensureSSHPortTreatsCreateErrorAsSuccessWhenPortNowExists(t *testing.T) {
	getCalls := 0
	svc := &fakeNodeService{
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			getCalls++
			node := &nodev1.ExternalNode{}
			if getCalls > 1 {
				node.Ports = []*nodev1.Port{{PortId: "port_ssh", PortNumber: 11640, ServerPort: 22}}
			}
			return &nodev1.GetNodeResponse{ExternalNode: node}, nil
		},
		openPortFn: func(_ *nodev1.OpenPortRequest) (*nodev1.OpenPortResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("Port 22 is already allocated for this client"))
		},
	}
	deps := startFakeServer(t, svc)
	reg := &register.DeviceRegistration{ExternalNodeID: "unode_abc", OrgID: "org_1"}

	if err := ensureSSHPort(context.Background(), terminal.New(), deps, &mockEnableSSHStore{}, reg, enableSSHOpts{interactive: true}); err != nil {
		t.Fatalf("ensureSSHPort: %v", err)
	}
	if svc.openCalls != 1 || getCalls != 2 {
		t.Fatalf("got OpenPort calls=%d GetNode calls=%d, want 1 and 2", svc.openCalls, getCalls)
	}
}

func Test_ensureSSHPortNonInteractiveDoesNotPrompt(t *testing.T) {
	svc := &fakeNodeService{
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{Ports: []*nodev1.Port{
				{PortId: "port_ssh", PortNumber: 11640, ServerPort: 22},
			}}}, nil
		},
	}
	deps := startFakeServer(t, svc)
	deps.prompter = mockSelector{
		inputLineFunc: func(*terminal.Terminal, string) (string, error) {
			t.Fatal("non-interactive mode must not prompt for input")
			return "", nil
		},
	}
	reg := &register.DeviceRegistration{ExternalNodeID: "unode_abc", OrgID: "org_1"}

	err := ensureSSHPort(context.Background(), terminal.New(), deps, &mockEnableSSHStore{}, reg, enableSSHOpts{
		linuxUsername: "ubuntu",
		sshPort:       22,
	})
	if err != nil {
		t.Fatalf("ensureSSHPort: %v", err)
	}
	if svc.openCalls != 0 {
		t.Fatalf("OpenPort called %d times, want 0", svc.openCalls)
	}
}

func Test_validateEnableSSHOpts(t *testing.T) {
	tests := []struct {
		name    string
		opts    enableSSHOpts
		wantErr string
	}{
		{name: "interactive", opts: enableSSHOpts{interactive: true}},
		{name: "non-interactive", opts: enableSSHOpts{linuxUsername: "ubuntu", sshPort: 22}},
		{name: "missing user", opts: enableSSHOpts{sshPort: 22}, wantErr: "--linux-user and --ssh-port are required"},
		{name: "missing port", opts: enableSSHOpts{linuxUsername: "ubuntu"}, wantErr: "--linux-user and --ssh-port are required"},
		{name: "invalid port", opts: enableSSHOpts{linuxUsername: "ubuntu", sshPort: 65536}, wantErr: "port must be between 1 and 65535"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnableSSHOpts(tt.opts)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateEnableSSHOpts: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func Test_promptLinuxUser(t *testing.T) {
	current := &user.User{Username: "current", HomeDir: "/current"}
	target := &user.User{Username: "ubuntu", HomeDir: "/home/ubuntu"}
	promptCalls := 0
	var promptLabel string
	deps := enableSSHDeps{
		currentUser: func() (*user.User, error) { return current, nil },
		prompter: mockSelector{
			inputLineFunc: func(_ *terminal.Terminal, label string) (string, error) {
				promptCalls++
				promptLabel = label
				return "", nil
			},
		},
		lookupUser: func(username string) (*user.User, error) {
			if username != "ubuntu" {
				t.Fatalf("lookup username = %q, want ubuntu", username)
			}
			return target, nil
		},
	}

	term := terminal.New()
	got, err := promptLinuxUser(term, deps)
	if err != nil || got != current {
		t.Fatalf("default user = %+v, err = %v", got, err)
	}
	if promptCalls != 1 {
		t.Fatalf("interactive prompt calls = %d, want 1", promptCalls)
	}
	if promptLabel != "Linux username (default current):" {
		t.Fatalf("prompt label = %q", promptLabel)
	}

	deps.prompter = mockSelector{
		inputLineFunc: func(_ *terminal.Terminal, _ string) (string, error) {
			promptCalls++
			return "ubuntu", nil
		},
	}
	got, err = promptLinuxUser(term, deps)
	if err != nil || got != target {
		t.Fatalf("prompted user = %+v, err = %v", got, err)
	}
	if promptCalls != 2 {
		t.Fatalf("interactive prompt calls = %d, want 2", promptCalls)
	}

	got, err = resolveLinuxUser(term, deps, enableSSHOpts{linuxUsername: "ubuntu", sshPort: 22})
	if err != nil || got != target {
		t.Fatalf("non-interactive user = %+v, err = %v", got, err)
	}
	if promptCalls != 2 {
		t.Fatalf("non-interactive mode should bypass the prompt; calls = %d", promptCalls)
	}
}

func Test_NewCmdEnableSSHExposesNonInteractiveFlags(t *testing.T) {
	cmd := NewCmdEnableSSH(terminal.New(), &mockEnableSSHStore{})
	for _, flagName := range []string{"linux-user", "ssh-port"} {
		if cmd.Flags().Lookup(flagName) == nil {
			t.Fatalf("enable-ssh should expose --%s for non-interactive mode", flagName)
		}
	}
}

// --- installCertAuthority ---

func Test_installCertAuthority(t *testing.T) {
	const (
		caKey = "ssh-ed25519 AAAAC3Nz dummyCA"
		node  = "unode_abc"
		luser = "ubuntu"
	)

	t.Run("WritesLine", func(t *testing.T) {
		u := tempUser(t)
		if err := installCertAuthority(u, caKey, node, luser); err != nil {
			t.Fatalf("installCertAuthority: %v", err)
		}
		want := `cert-authority,principals="brev:v1:vm:unode_abc:login:ubuntu" ssh-ed25519 AAAAC3Nz dummyCA`
		if result := readAuthorizedKeys(t, u); !strings.Contains(result, want) {
			t.Errorf("expected cert-authority line not found:\n%s", result)
		}
	})

	t.Run("Idempotent", func(t *testing.T) {
		u := tempUser(t)
		for i := 0; i < 2; i++ {
			if err := installCertAuthority(u, caKey, node, luser); err != nil {
				t.Fatalf("installCertAuthority #%d: %v", i+1, err)
			}
		}
		result := readAuthorizedKeys(t, u)
		if n := strings.Count(result, "cert-authority"); n != 1 {
			t.Errorf("expected 1 cert-authority line, got %d:\n%s", n, result)
		}
	})

	t.Run("PreservesExistingKeys", func(t *testing.T) {
		u := tempUser(t)
		sshDir := filepath.Join(u.HomeDir, ".ssh")
		if err := os.MkdirAll(sshDir, 0o700); err != nil {
			t.Fatal(err)
		}
		original := "ssh-rsa EXISTING user@host\n"
		if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(original), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := installCertAuthority(u, caKey, node, luser); err != nil {
			t.Fatalf("installCertAuthority: %v", err)
		}

		result := readAuthorizedKeys(t, u)
		if !strings.Contains(result, "ssh-rsa EXISTING user@host") {
			t.Errorf("existing key was removed:\n%s", result)
		}
		if !strings.Contains(result, "cert-authority") {
			t.Errorf("cert-authority line not written:\n%s", result)
		}
	})

	t.Run("EmptyKeyErrors", func(t *testing.T) {
		if err := installCertAuthority(tempUser(t), "", node, luser); err == nil {
			t.Error("expected error for empty CA key")
		}
	})
}

func Test_resolveLegacySSHPortNonInteractiveUsesProvidedPort(t *testing.T) {
	svc := &fakeNodeService{
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{}}, nil
		},
	}
	deps := startFakeServer(t, svc)
	node := &nodev1.ExternalNode{Ports: []*nodev1.Port{
		{PortId: "port_ssh", PortNumber: 11640, ServerPort: 22},
	}}
	reg := &register.DeviceRegistration{ExternalNodeID: "unode_abc", OrgID: "org_1"}

	portID, err := resolveLegacySSHPort(
		context.Background(),
		terminal.New(),
		deps,
		&mockEnableSSHStore{},
		reg,
		node,
		enableSSHOpts{linuxUsername: "ubuntu", sshPort: 22},
	)
	if err != nil {
		t.Fatalf("resolveLegacySSHPort: %v", err)
	}
	if portID != "port_ssh" {
		t.Fatalf("port ID = %q, want port_ssh", portID)
	}
	if svc.openCalls != 0 {
		t.Fatalf("OpenPort called %d times, want 0", svc.openCalls)
	}
}

func Test_enableSSHNonInteractiveUsesProvidedInputs(t *testing.T) {
	const caKey = "ssh-ed25519 AAAAC3Nz dummyCA"
	svc := &fakeNodeService{
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{ExternalNode: &nodev1.ExternalNode{Ports: []*nodev1.Port{
				{PortId: "port_ssh", PortNumber: 11640, ServerPort: 22},
			}}}, nil
		},
	}
	deps := startFakeServer(t, svc)
	targetUser := &user.User{Username: "ubuntu", HomeDir: t.TempDir()}
	promptCalls := 0
	deps.prompter = mockSelector{
		inputLineFunc: func(*terminal.Terminal, string) (string, error) {
			promptCalls++
			t.Fatal("non-interactive mode must not prompt for input")
			return "", nil
		},
	}
	deps.currentUser = func() (*user.User, error) {
		t.Fatal("non-interactive mode must not resolve the current Linux user")
		return nil, nil
	}
	deps.lookupUser = func(username string) (*user.User, error) {
		if username != "ubuntu" {
			t.Fatalf("lookup username = %q, want ubuntu", username)
		}
		return targetUser, nil
	}
	reg := &register.DeviceRegistration{
		ExternalNodeID:       "unode_abc",
		OrgID:                "org_1",
		CertificateAuthority: caKey,
	}

	err := enableSSH(context.Background(), terminal.New(), deps, &mockEnableSSHStore{}, reg, enableSSHOpts{
		linuxUsername: "ubuntu",
		sshPort:       22,
	})
	if err != nil {
		t.Fatalf("enableSSH: %v", err)
	}
	if promptCalls != 0 {
		t.Fatalf("Linux username prompt calls = %d, want 0", promptCalls)
	}
	if svc.openCalls != 0 {
		t.Fatalf("OpenPort called %d times, want 0", svc.openCalls)
	}
	wantPrincipal := `principals="brev:v1:vm:unode_abc:login:ubuntu"`
	if authorizedKeys := readAuthorizedKeys(t, targetUser); !strings.Contains(authorizedKeys, wantPrincipal) {
		t.Fatalf("authorized_keys missing %s:\n%s", wantPrincipal, authorizedKeys)
	}
}

func Test_enableSSH_LegacyNodeFallsBackToKeys(t *testing.T) {
	svc := &fakeNodeService{
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: "unode_legacy",
					// No sshprovider label — legacy node.
					Labels: map[string]string{},
					Ports: []*nodev1.Port{{
						PortId:     "port_ssh",
						Protocol:   nodev1.PortProtocol_PORT_PROTOCOL_TCP,
						PortNumber: 22,
					}},
				},
			}, nil
		},
	}
	deps := startFakeServer(t, svc)
	// Real username (the legacy path does a system user.Lookup) but temp
	// HomeDir so authorized_keys operations never touch the developer's
	// real file.
	realUser, uerr := user.Current()
	if uerr != nil {
		t.Fatalf("user.Current: %v", uerr)
	}
	tempUser := &user.User{Username: realUser.Username, HomeDir: t.TempDir()}
	deps.currentUser = func() (*user.User, error) { return tempUser, nil }

	reg := &register.DeviceRegistration{
		DisplayName:    "legacy-node",
		ExternalNodeID: "unode_legacy",
		OrgID:          "org_1",
	}

	term := terminal.New()
	if err := enableSSH(context.Background(), term, deps, &mockEnableSSHStore{}, reg, enableSSHOpts{interactive: true}); err != nil {
		t.Fatalf("allowSSH failed: %v", err)
	}

	// Legacy flow must grant SSH access (reflexive grant).
	if svc.grantCalls == 0 {
		t.Error("expected GrantNodeSSHAccess to be called for legacy node")
	}

	// No cert-authority line may be written for a legacy node.
	authKeysPath := filepath.Join(tempUser.HomeDir, ".ssh", "authorized_keys")
	data, readErr := os.ReadFile(authKeysPath) // #nosec G304
	if readErr == nil {
		if strings.Contains(string(data), "brev:v1:vm:unode_legacy") {
			t.Errorf("legacy node must not write a cert-authority line:\n%s", string(data))
		}
	}
}
