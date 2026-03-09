package grantssh

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

// mock types for grantSSHDeps interfaces

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

func (m *mockGrantSSHStore) GetAccessToken() (string, error) { return m.token, nil }

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

func (f *fakeNodeService) OpenPort(_ context.Context, req *connect.Request[nodev1.OpenPortRequest]) (*connect.Response[nodev1.OpenPortResponse], error) {
	return connect.NewResponse(&nodev1.OpenPortResponse{
		Port: &nodev1.Port{
			PortId:     "port_ssh",
			Protocol:   req.Msg.GetProtocol(),
			PortNumber: req.Msg.GetPortNumber(),
			ServerPort: req.Msg.GetPortNumber(),
		},
	}), nil
}

func (f *fakeNodeService) GrantNodeSSHAccess(_ context.Context, req *connect.Request[nodev1.GrantNodeSSHAccessRequest]) (*connect.Response[nodev1.GrantNodeSSHAccessResponse], error) {
	resp, err := f.grantSSHFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func testGrantSSHDeps(t *testing.T, svc *fakeNodeService, regStore register.RegistrationStore) (grantSSHDeps, *httptest.Server) {
	t.Helper()

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)

	return grantSSHDeps{
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

func Test_runGrantSSH_NotRegistered(t *testing.T) {
	regStore := &mockRegistrationStore{} // no registration

	store := &mockGrantSSHStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
		token: "tok",
	}

	svc := &fakeNodeService{}
	deps, server := testGrantSSHDeps(t, svc, regStore)
	defer server.Close()

	term := terminal.New()
	// No registration and no nodes from ListNodes → should fail
	err := runGrantSSH(context.Background(), term, store, deps, "testuser")
	if err == nil {
		t.Fatal("expected error when not registered and no nodes available")
	}
}

func Test_runGrantSSH_HappyPath(t *testing.T) {
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
		user:  &entity.User{ID: "user_1", PublicKey: "ssh-ed25519 testkey"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
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

	term := terminal.New()
	err := runGrantSSH(context.Background(), term, store, deps, "ubuntu")
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
	if gotReq.GetLinuxUser() != "ubuntu" {
		t.Errorf("expected linux user ubuntu, got %s", gotReq.GetLinuxUser())
	}
}

func Test_runGrantSSH_RPCFailure(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
		},
	}

	store := &mockGrantSSHStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
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

	term := terminal.New()
	err := runGrantSSH(context.Background(), term, store, deps, "testuser")
	if err == nil {
		t.Fatal("expected error when GrantNodeSSHAccess fails")
	}
}

func Test_runGrantSSH_NoOtherMembers(t *testing.T) {
	regStore := &mockRegistrationStore{
		reg: &register.DeviceRegistration{
			ExternalNodeID: "unode_abc",
			DisplayName:    "My Spark",
			OrgID:          "org_123",
		},
	}

	store := &mockGrantSSHStore{
		user:  &entity.User{ID: "user_1"},
		org:   &entity.Organization{ID: "org_123", Name: "TestOrg"},
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
	err := runGrantSSH(context.Background(), term, store, deps, "testuser")
	if err == nil {
		t.Fatal("expected error when no other members exist")
	}
}
