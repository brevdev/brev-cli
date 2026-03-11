package revokessh

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

// --- mock types ---

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
	org   *entity.Organization
	users map[string]*entity.User
}

func (m *mockRevokeSSHStore) GetAccessToken() (string, error) { return m.token, nil }

func (m *mockRevokeSSHStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return m.org, nil
}

func (m *mockRevokeSSHStore) GetUserByID(userID string) (*entity.User, error) {
	if m.users != nil {
		if u, ok := m.users[userID]; ok {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user %s not found", userID)
}

func (m *mockRevokeSSHStore) ListOrganizations() ([]entity.Organization, error) {
	if m.org == nil {
		return nil, nil
	}
	return []entity.Organization{*m.org}, nil
}

func (m *mockRevokeSSHStore) GetOrganizationsByName(name string) ([]entity.Organization, error) {
	if m.org != nil && m.org.Name == name {
		return []entity.Organization{*m.org}, nil
	}
	return nil, nil
}

func (m *mockRevokeSSHStore) GetOrgRoleAttachments(_ string) ([]entity.OrgRoleAttachment, error) {
	return nil, nil
}

// fakeNodeService implements the server side of ExternalNodeService for testing.
type fakeNodeService struct {
	nodev1connect.UnimplementedExternalNodeServiceHandler
	getNodeFn   func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error)
	revokeSSHFn func(*nodev1.RevokeNodeSSHAccessRequest) (*nodev1.RevokeNodeSSHAccessResponse, error)
}

func (f *fakeNodeService) GetNode(_ context.Context, req *connect.Request[nodev1.GetNodeRequest]) (*connect.Response[nodev1.GetNodeResponse], error) {
	resp, err := f.getNodeFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (f *fakeNodeService) RevokeNodeSSHAccess(_ context.Context, req *connect.Request[nodev1.RevokeNodeSSHAccessRequest]) (*connect.Response[nodev1.RevokeNodeSSHAccessResponse], error) {
	resp, err := f.revokeSSHFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// --- helpers ---

func baseDeps(t *testing.T, svc *fakeNodeService, regStore register.RegistrationStore) (revokeSSHDeps, *httptest.Server) {
	t.Helper()

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)

	return revokeSSHDeps{
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

// --- tests ---

func Test_runRevokeSSH_NotRegistered(t *testing.T) {
	regStore := &mockRegistrationStore{} // no registration
	store := &mockRevokeSSHStore{
		token: "tok",
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
	}
	svc := &fakeNodeService{}

	deps, server := baseDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := revokeSSHOpts{interactive: true, skipConfirm: true}
	err := runRevokeSSH(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when not registered and no nodes available")
	}
}

func Test_runRevokeSSH_NoSSHAccess(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
		},
	}
	store := &mockRevokeSSHStore{
		token: "tok",
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
	}

	svc := &fakeNodeService{
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: "unode_abc",
					SshAccess:      nil, // no SSH access
				},
			}, nil
		},
	}

	deps, server := baseDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := revokeSSHOpts{interactive: true, skipConfirm: true}
	err := runRevokeSSH(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("expected nil error for no SSH access, got: %v", err)
	}
}

func Test_runRevokeSSH_RevokeAccess(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
		},
	}
	store := &mockRevokeSSHStore{
		token: "tok",
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		users: map[string]*entity.User{
			"user_2": {ID: "user_2", Name: "Alice"},
		},
	}

	var gotReq *nodev1.RevokeNodeSSHAccessRequest
	svc := &fakeNodeService{
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: "unode_abc",
					SshAccess: []*nodev1.SSHAccess{
						{UserId: "user_2", LinuxUser: "ubuntu"},
					},
				},
			}, nil
		},
		revokeSSHFn: func(req *nodev1.RevokeNodeSSHAccessRequest) (*nodev1.RevokeNodeSSHAccessResponse, error) {
			gotReq = req
			return &nodev1.RevokeNodeSSHAccessResponse{}, nil
		},
	}

	deps, server := baseDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := revokeSSHOpts{interactive: true, skipConfirm: true}
	err := runRevokeSSH(context.Background(), term, store, opts, deps)
	if err != nil {
		t.Fatalf("runRevokeSSH failed: %v", err)
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
	if gotReq.GetLinuxUser() != "ubuntu" {
		t.Errorf("expected linux user ubuntu, got %s", gotReq.GetLinuxUser())
	}
}

func Test_runRevokeSSH_RPCFailure(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
		},
	}
	store := &mockRevokeSSHStore{
		token: "tok",
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
	}

	svc := &fakeNodeService{
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: "unode_abc",
					SshAccess: []*nodev1.SSHAccess{
						{UserId: "user_3", LinuxUser: "testuser"},
					},
				},
			}, nil
		},
		revokeSSHFn: func(_ *nodev1.RevokeNodeSSHAccessRequest) (*nodev1.RevokeNodeSSHAccessResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, nil)
		},
	}

	deps, server := baseDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	opts := revokeSSHOpts{interactive: true, skipConfirm: true}
	err := runRevokeSSH(context.Background(), term, store, opts, deps)
	if err == nil {
		t.Fatal("expected error when RevokeNodeSSHAccess fails")
	}
}
