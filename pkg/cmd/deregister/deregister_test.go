package deregister

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os/user"
	"strings"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/sudo"
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
	listNodesFn  func(*nodev1.ListNodesRequest) (*nodev1.ListNodesResponse, error)
	getNodeFn    func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error)
}

func (f *fakeNodeService) GetNode(_ context.Context, req *connect.Request[nodev1.GetNodeRequest]) (*connect.Response[nodev1.GetNodeResponse], error) {
	if f.getNodeFn == nil {
		// Default: certauth node (matches registration on this branch).
		return connect.NewResponse(&nodev1.GetNodeResponse{
			ExternalNode: &nodev1.ExternalNode{
				ExternalNodeId: req.Msg.GetExternalNodeId(),
				Labels:         map[string]string{"sshprovider": "certauth"},
			},
		}), nil
	}
	resp, err := f.getNodeFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (f *fakeNodeService) RemoveNode(_ context.Context, req *connect.Request[nodev1.RemoveNodeRequest]) (*connect.Response[nodev1.RemoveNodeResponse], error) {
	resp, err := f.removeNodeFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (f *fakeNodeService) ListNodes(_ context.Context, req *connect.Request[nodev1.ListNodesRequest]) (*connect.Response[nodev1.ListNodesResponse], error) {
	resp, err := f.listNodesFn(req.Msg)
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
	return nil, fmt.Errorf("unexpected call to Load")
}

func (m *mockRegistrationStore) LoadAll() (*register.DeviceRegistration, error) {
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
	removed bool
}

func (m *mockSSHKeyRemover) RemoveCertAuthority(_ *user.User, _, _ string) (bool, error) {
	m.called = true
	return m.removed, m.err
}

type mockLegacyKeyRemover struct {
	called  bool
	err     error
	removed []string
}

func (m *mockLegacyKeyRemover) RemoveBrevKeys(_ *user.User) ([]string, error) {
	m.called = true
	return m.removed, m.err
}

// testDeregisterDeps returns deps with all side-effects stubbed. The
// prompter defaults to confirming all prompts.
func registeredReg() *register.DeviceRegistration {
	return &register.DeviceRegistration{
		ExternalNodeID: "unode_abc",
		DisplayName:    "My Spark",
		OrgID:          "org_123",
		DeviceID:       "dev-uuid",
		Status:         register.RegistrationStatusRegistered,
	}
}

// runDeregisterCase absorbs scaffolding the tests repeat: the standard
// store, deps, server lifecycle, terminal, and the non-interactive invocation.
func runDeregisterCase(t *testing.T, regStore *mockRegistrationStore, svc *fakeNodeService, mutate ...func(*deregisterDeps)) error {
	t.Helper()
	store := &mockDeregisterStore{user: &entity.User{ID: "user_1"}, token: "tok"}
	deps, server := testDeregisterDeps(t, svc, regStore)
	defer server.Close()
	for _, m := range mutate {
		m(&deps)
	}
	return runDeregister(context.Background(), terminal.New(), store, deps, false)
}

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
		gater:             sudo.CachedGater{},
		netbird:           &mockNetBirdManager{},
		nodeClients:       mockNodeClientFactory{serverURL: server.URL},
		registrationStore: regStore,
		sshKeys:           &mockSSHKeyRemover{},
		legacyKeys:        &mockLegacyKeyRemover{},
	}, server
}

func Test_runDeregister_HappyPath(t *testing.T) {
	regStore := &mockRegistrationStore{reg: registeredReg()}

	var gotNodeID string
	svc := &fakeNodeService{
		removeNodeFn: func(req *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			gotNodeID = req.GetExternalNodeId()
			return &nodev1.RemoveNodeResponse{}, nil
		},
	}

	err := runDeregisterCase(t, regStore, svc)
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
	regStore := &mockRegistrationStore{reg: registeredReg()}

	svc := &fakeNodeService{}
	err := runDeregisterCase(t, regStore, svc, func(d *deregisterDeps) {
		d.prompter = mockSelector{fn: func(_ string, _ []string) string { return "No, cancel" }}
	})
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

	svc := &fakeNodeService{}
	err := runDeregisterCase(t, regStore, svc)
	if err == nil {
		t.Fatal("expected error when not registered")
	}
}

func Test_runDeregister_RemoveNodeFails(t *testing.T) {
	regStore := &mockRegistrationStore{reg: registeredReg()}

	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, nil)
		},
	}

	err := runDeregisterCase(t, regStore, svc)
	if err == nil {
		t.Fatal("expected error when RemoveNode fails")
	}

	exists, err := regStore.Exists()
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Error("registration should still exist when RemoveNode fails")
	}
}

func Test_runDeregister_RemoveNodeNotFound_ProceedsCleanup(t *testing.T) {
	regStore := &mockRegistrationStore{reg: registeredReg()}

	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, nil)
		},
	}

	err := runDeregisterCase(t, regStore, svc)
	if err != nil {
		t.Fatalf("NotFound should be treated as success (node already gone), got: %v", err)
	}

	exists, err := regStore.Exists()
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if exists {
		t.Error("expected local registration to be deleted even when RemoveNode returns NotFound")
	}
}

func Test_runDeregister_PendingRegistration(t *testing.T) {
	const deviceID = "dev-uuid-pending"
	reg := &register.DeviceRegistration{
		DisplayName: "My Spark",
		OrgID:       "org_123",
		DeviceID:    deviceID,
		Status:      register.RegistrationStatusPending,
	}

	tests := []struct {
		name          string
		listNodesFn   func(*nodev1.ListNodesRequest) (*nodev1.ListNodesResponse, error)
		removeNodeFn  func(*nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error)
		wantRemovedID string // empty = RemoveNode must not be called
		wantRunErr    string // non-empty = runDeregister must fail with this substring
	}{
		{
			name: "no backend node matches: cleans up locally without RemoveNode",
			listNodesFn: func(*nodev1.ListNodesRequest) (*nodev1.ListNodesResponse, error) {
				return &nodev1.ListNodesResponse{}, nil
			},
			removeNodeFn: func(req *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
				return nil, fmt.Errorf("RemoveNode should not be called with empty ID %q", req.GetExternalNodeId())
			},
		},
		{
			name: "backend node recovered by device ID: removed",
			listNodesFn: func(req *nodev1.ListNodesRequest) (*nodev1.ListNodesResponse, error) {
				return &nodev1.ListNodesResponse{
					Items: []*nodev1.ExternalNode{
						{ExternalNodeId: "unode_other", DeviceId: "dev-different"},
						{ExternalNodeId: "unode_recovered", DeviceId: deviceID},
					},
				}, nil
			},
			removeNodeFn: func(req *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
				return &nodev1.RemoveNodeResponse{}, nil
			},
			wantRemovedID: "unode_recovered",
		},
		{
			name: "ListNodes failure is fatal: deregister aborts, local state kept",
			listNodesFn: func(*nodev1.ListNodesRequest) (*nodev1.ListNodesResponse, error) {
				return nil, connect.NewError(connect.CodeInternal, nil)
			},
			wantRunErr: "failed to find pending node by device ID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regStore := &mockRegistrationStore{reg: reg}
			store := &mockDeregisterStore{user: &entity.User{ID: "user_1"}, token: "tok"}

			var gotOrgID string
			var removedNodeID string
			svc := &fakeNodeService{
				listNodesFn: func(req *nodev1.ListNodesRequest) (*nodev1.ListNodesResponse, error) {
					gotOrgID = req.GetOrganizationId()
					return tt.listNodesFn(req)
				},
				removeNodeFn: func(req *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
					removedNodeID = req.GetExternalNodeId()
					return tt.removeNodeFn(req)
				},
			}

			deps, server := testDeregisterDeps(t, svc, regStore)
			defer server.Close()

			term := terminal.New()
			err := runDeregister(context.Background(), term, store, deps, false)
			if tt.wantRunErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantRunErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantRunErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("deregister failed: %v", err)
			}

			if gotOrgID != "org_123" {
				t.Errorf("expected ListNodes scoped to org_123, got %q", gotOrgID)
			}
			if tt.wantRemovedID == "" {
				if removedNodeID != "" {
					t.Errorf("RemoveNode should not be called, got %q", removedNodeID)
				}
			} else if removedNodeID != tt.wantRemovedID {
				t.Errorf("expected RemoveNode called with %q, got %q", tt.wantRemovedID, removedNodeID)
			}

			exists, _ := regStore.Exists()
			if exists {
				t.Error("expected local registration to be deleted")
			}
		})
	}
}

func Test_findNodeByDeviceID_PaginatesUntilFound(t *testing.T) {
	const deviceID = "dev-uuid-pending"
	deps, server := testDeregisterDeps(t, &fakeNodeService{
		listNodesFn: func(req *nodev1.ListNodesRequest) (*nodev1.ListNodesResponse, error) {
			switch req.GetPageParams().GetPageToken() {
			case "":
				return &nodev1.ListNodesResponse{
					Items:         []*nodev1.ExternalNode{{ExternalNodeId: "unode_page1", DeviceId: "dev-other"}},
					NextPageToken: "page-2",
				}, nil
			case "page-2":
				return &nodev1.ListNodesResponse{
					Items:         []*nodev1.ExternalNode{{ExternalNodeId: "unode_page2", DeviceId: deviceID}},
					NextPageToken: "",
				}, nil
			default:
				return nil, fmt.Errorf("unexpected page token %q", req.GetPageParams().GetPageToken())
			}
		},
	}, &mockRegistrationStore{})
	defer server.Close()

	nodeID, err := findNodeByDeviceID(context.Background(), storeToken("tok"), deps, "org_123", deviceID)
	if err != nil {
		t.Fatalf("findNodeByDeviceID failed: %v", err)
	}
	if nodeID != "unode_page2" {
		t.Errorf("expected node from page 2, got %q", nodeID)
	}
}

type storeToken string

func (t storeToken) GetAccessToken() (string, error) { return string(t), nil }

func Test_runDeregister_AlwaysUninstallsNetbird(t *testing.T) {
	regStore := &mockRegistrationStore{reg: registeredReg()}

	netbird := &mockNetBirdManager{}
	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return &nodev1.RemoveNodeResponse{}, nil
		},
	}

	err := runDeregisterCase(t, regStore, svc, func(d *deregisterDeps) { d.netbird = netbird })
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
			regStore := &mockRegistrationStore{reg: registeredReg()}

			svc := &fakeNodeService{
				removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
					return &nodev1.RemoveNodeResponse{}, nil
				},
			}

			err := runDeregisterCase(t, regStore, svc, func(d *deregisterDeps) { d.sshKeys = tt.sshKeys })
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

func Test_runDeregister_LegacyNodeRemovesKeys(t *testing.T) {
	regStore := &mockRegistrationStore{reg: registeredReg()}
	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return &nodev1.RemoveNodeResponse{}, nil
		},
		getNodeFn: func(req *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return &nodev1.GetNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: req.GetExternalNodeId(),
					// No sshprovider label — legacy node.
					Labels: map[string]string{},
				},
			}, nil
		},
	}

	certMock := &mockSSHKeyRemover{}
	legacyMock := &mockLegacyKeyRemover{removed: []string{"ssh-rsa OLD user@host"}}

	err := runDeregisterCase(t, regStore, svc, func(d *deregisterDeps) {
		d.sshKeys = certMock
		d.legacyKeys = legacyMock
	})
	if err != nil {
		t.Fatalf("runDeregister failed: %v", err)
	}

	if !legacyMock.called {
		t.Error("expected RemoveBrevKeys to be called for legacy node")
	}
	if certMock.called {
		t.Error("expected RemoveCertAuthority NOT to be called for legacy node")
	}
}

func Test_runDeregister_CertAuthNodeRemovesCertAuthority(t *testing.T) {
	regStore := &mockRegistrationStore{reg: registeredReg()}
	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return &nodev1.RemoveNodeResponse{}, nil
		},
	} // default getNodeFn returns a certauth node

	certMock := &mockSSHKeyRemover{removed: true}
	legacyMock := &mockLegacyKeyRemover{}

	err := runDeregisterCase(t, regStore, svc, func(d *deregisterDeps) {
		d.sshKeys = certMock
		d.legacyKeys = legacyMock
	})
	if err != nil {
		t.Fatalf("runDeregister failed: %v", err)
	}

	if !certMock.called {
		t.Error("expected RemoveCertAuthority to be called for certauth node")
	}
	if legacyMock.called {
		t.Error("expected RemoveBrevKeys NOT to be called for certauth node")
	}
}

func Test_runDeregister_NodeLookupFailure_CleansBoth(t *testing.T) {
	regStore := &mockRegistrationStore{reg: registeredReg()}
	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return &nodev1.RemoveNodeResponse{}, nil
		},
		getNodeFn: func(_ *nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("backend down"))
		},
	}

	certMock := &mockSSHKeyRemover{removed: true}
	legacyMock := &mockLegacyKeyRemover{removed: []string{"ssh-rsa OLD"}}

	err := runDeregisterCase(t, regStore, svc, func(d *deregisterDeps) {
		d.sshKeys = certMock
		d.legacyKeys = legacyMock
	})
	if err != nil {
		t.Fatalf("runDeregister failed: %v", err)
	}

	if !certMock.called {
		t.Error("expected RemoveCertAuthority to be called on lookup failure")
	}
	if !legacyMock.called {
		t.Error("expected RemoveBrevKeys to be called on lookup failure")
	}
}
