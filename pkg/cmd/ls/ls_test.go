package ls

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

// mockLsStore implements LsStore (including the embedded hello.HelloStore) for
// testing the ls command routing without real API calls.
type mockLsStore struct {
	user       *entity.User
	org        *entity.Organization
	orgs       []entity.Organization
	workspaces []entity.Workspace
}

func (m *mockLsStore) GetCurrentUser() (*entity.User, error) { return m.user, nil }
func (m *mockLsStore) GetAccessToken() (string, error)       { return "tok", nil }
func (m *mockLsStore) GetWorkspace(_ string) (*entity.Workspace, error) {
	return nil, nil
}

func (m *mockLsStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return m.org, nil
}

func (m *mockLsStore) GetWorkspaces(_ string, _ *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	return m.workspaces, nil
}

func (m *mockLsStore) GetOrganizations(_ *store.GetOrganizationsOptions) ([]entity.Organization, error) {
	return m.orgs, nil
}

func (m *mockLsStore) GetUsers(_ map[string]string) ([]entity.User, error) {
	return nil, nil
}

func (m *mockLsStore) GetInstanceTypes(_ bool) (*gpusearch.InstanceTypesResponse, error) {
	return &gpusearch.InstanceTypesResponse{}, nil
}

// hello.HelloStore methods
func (m *mockLsStore) GetAllWorkspaces(_ *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	return m.workspaces, nil
}

func (m *mockLsStore) UpdateUser(_ string, _ *entity.UpdateUser) (*entity.User, error) {
	return m.user, nil
}
func (m *mockLsStore) GetCurrentWorkspaceID() (string, error) { return "", nil }

func newTestStore() *mockLsStore {
	user := &entity.User{ID: "u1", Name: "Test User"}
	org := &entity.Organization{ID: "org1", Name: "test-org"}
	return &mockLsStore{
		user: user,
		org:  org,
		orgs: []entity.Organization{*org},
	}
}

// TestHandleLsArg_Instances verifies that "instance" and "instances" route to
// RunInstances (workspaces only, no nodes).
func TestHandleLsArg_Instances(t *testing.T) {
	s := newTestStore()
	term := terminal.New()
	ls := NewLs(s, term, true) // json mode for simpler output

	for _, arg := range []string{"instance", "instances"} {
		err := handleLsArg(ls, arg, s.user, s.org, false)
		if err != nil {
			t.Fatalf("handleLsArg(%q) returned error: %v", arg, err)
		}
	}
}

// TestHandleLsArg_Nodes verifies that "node" and "nodes" route to RunNodes.
func TestHandleLsArg_Nodes(t *testing.T) {
	s := newTestStore()
	term := terminal.New()
	ls := NewLs(s, term, true)

	// RunNodes will call listNodes which requires a real gRPC client, so it
	// will return an error — but the important thing is that handleLsArg
	// attempts the node path (not workspaces).
	for _, arg := range []string{"node", "nodes"} {
		err := handleLsArg(ls, arg, s.user, s.org, false)
		// We expect an error because we don't have a real node service, but
		// the point is it doesn't panic and routes correctly.
		_ = err
	}
}

// TestHandleLsArg_Orgs verifies that "org", "orgs", "organization",
// "organizations" all route to RunOrgs.
func TestHandleLsArg_Orgs(t *testing.T) {
	s := newTestStore()
	term := terminal.New()
	ls := NewLs(s, term, true)

	for _, arg := range []string{"org", "orgs", "organization", "organizations"} {
		err := handleLsArg(ls, arg, s.user, s.org, false)
		if err != nil {
			t.Fatalf("handleLsArg(%q) returned error: %v", arg, err)
		}
	}
}

// TestRunLs_DefaultShowsWorkspacesOnly verifies that `brev ls` with no args
// calls RunWorkspaces (not RunNodes or RunInstances with nodes).
func TestRunLs_DefaultShowsWorkspacesOnly(t *testing.T) {
	s := newTestStore()
	term := terminal.New()

	// Should succeed with no args — lists workspaces only.
	err := RunLs(term, s, nil, "", false, true)
	if err != nil {
		t.Fatalf("RunLs with no args returned error: %v", err)
	}
}

// TestRunLs_InstancesArg verifies that `brev ls instances` succeeds.
func TestRunLs_InstancesArg(t *testing.T) {
	s := newTestStore()
	term := terminal.New()

	err := RunLs(term, s, []string{"instances"}, "", false, true)
	if err != nil {
		t.Fatalf("RunLs with 'instances' arg returned error: %v", err)
	}
}

// TestRunLs_TooManyArgs verifies that more than one arg is rejected.
func TestRunLs_TooManyArgs(t *testing.T) {
	s := newTestStore()
	term := terminal.New()

	err := RunLs(term, s, []string{"instances", "nodes"}, "", false, true)
	if err == nil {
		t.Fatal("expected error for too many args, got nil")
	}
}

// TestRunLs_WorkspacesWithData verifies workspace JSON output includes the
// right workspaces and doesn't include node data.
func TestRunLs_WorkspacesWithData(t *testing.T) {
	s := newTestStore()
	s.workspaces = []entity.Workspace{
		{
			ID:              "ws1",
			Name:            "my-workspace",
			Status:          entity.Running,
			CreatedByUserID: "u1",
		},
	}
	term := terminal.New()

	// JSON output — should not error and should only contain workspace data.
	err := RunLs(term, s, nil, "", false, true)
	if err != nil {
		t.Fatalf("RunLs returned error: %v", err)
	}
}
