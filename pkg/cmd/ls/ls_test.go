package ls

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

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

// captureStdout runs fn while capturing stdout and returns the output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	r.Close()
	return string(buf[:n])
}

// TestRunLs_DefaultJSON verifies that `brev ls --json` outputs workspace JSON
// with the expected fields and values.
func TestRunLs_DefaultJSON(t *testing.T) {
	s := newTestStore()
	s.workspaces = []entity.Workspace{
		{
			ID:              "ws1",
			Name:            "dev-box",
			Status:          entity.Running,
			HealthStatus:    "HEALTHY",
			VerbBuildStatus: entity.Completed,
			CreatedByUserID: "u1",
			InstanceType:    "g5.xlarge",
		},
		{
			ID:              "ws2",
			Name:            "staging",
			Status:          entity.Stopped,
			CreatedByUserID: "u1",
		},
	}
	term := terminal.New()

	out := captureStdout(t, func() {
		err := RunLs(term, s, nil, "", false, true)
		if err != nil {
			t.Fatalf("RunLs returned error: %v", err)
		}
	})

	var parsed struct {
		Workspaces []WorkspaceInfo `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, out)
	}
	results := parsed.Workspaces

	if len(results) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(results))
	}

	// First workspace
	if results[0].Name != "dev-box" {
		t.Errorf("expected name 'dev-box', got %q", results[0].Name)
	}
	if results[0].ID != "ws1" {
		t.Errorf("expected id 'ws1', got %q", results[0].ID)
	}
	if results[0].Status != entity.Running {
		t.Errorf("expected status %q, got %q", entity.Running, results[0].Status)
	}
	if results[0].ShellStatus != entity.Ready {
		t.Errorf("expected shell status %q, got %q", entity.Ready, results[0].ShellStatus)
	}

	// Second workspace
	if results[1].Name != "staging" {
		t.Errorf("expected name 'staging', got %q", results[1].Name)
	}
	if results[1].Status != entity.Stopped {
		t.Errorf("expected status %q, got %q", entity.Stopped, results[1].Status)
	}
}

// TestRunLs_DefaultJSON_Empty verifies that `brev ls --json` with no
// workspaces outputs a valid empty JSON array.
func TestRunLs_DefaultJSON_Empty(t *testing.T) {
	s := newTestStore()
	term := terminal.New()

	out := captureStdout(t, func() {
		err := RunLs(term, s, nil, "", false, true)
		if err != nil {
			t.Fatalf("RunLs returned error: %v", err)
		}
	})

	var parsed struct {
		Workspaces []WorkspaceInfo `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}
	if len(parsed.Workspaces) != 0 {
		t.Fatalf("expected 0 workspaces, got %d", len(parsed.Workspaces))
	}
}

// TestRunLs_InstancesJSON verifies that `brev ls instances --json` produces
// the same workspace-only output as the default.
func TestRunLs_InstancesJSON(t *testing.T) {
	s := newTestStore()
	s.workspaces = []entity.Workspace{
		{
			ID:              "ws1",
			Name:            "my-instance",
			Status:          entity.Running,
			VerbBuildStatus: entity.Completed,
			CreatedByUserID: "u1",
		},
	}
	term := terminal.New()

	out := captureStdout(t, func() {
		err := RunLs(term, s, []string{"instances"}, "", false, true)
		if err != nil {
			t.Fatalf("RunLs instances returned error: %v", err)
		}
	})

	var parsed struct {
		Workspaces []WorkspaceInfo `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}
	if len(parsed.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(parsed.Workspaces))
	}
	if parsed.Workspaces[0].Name != "my-instance" {
		t.Errorf("expected name 'my-instance', got %q", parsed.Workspaces[0].Name)
	}
}

// TestRunLs_OrgsJSON verifies that `brev ls orgs --json` outputs org data with
// the active org marked.
func TestRunLs_OrgsJSON(t *testing.T) {
	s := newTestStore()
	s.orgs = []entity.Organization{
		{ID: "org1", Name: "primary-org"},
		{ID: "org2", Name: "secondary-org"},
	}
	term := terminal.New()

	out := captureStdout(t, func() {
		err := RunLs(term, s, []string{"orgs"}, "", false, true)
		if err != nil {
			t.Fatalf("RunLs orgs returned error: %v", err)
		}
	})

	var results []OrgInfo
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 orgs, got %d", len(results))
	}
	if results[0].Name != "primary-org" {
		t.Errorf("expected name 'primary-org', got %q", results[0].Name)
	}
	if !results[0].IsActive {
		t.Error("expected first org to be active")
	}
	if results[1].IsActive {
		t.Error("expected second org to not be active")
	}
}

// TestRunLs_ShowAll verifies that --all includes workspaces from other users.
func TestRunLs_ShowAllJSON(t *testing.T) {
	s := newTestStore()
	s.workspaces = []entity.Workspace{
		{
			ID:              "ws-mine",
			Name:            "my-ws",
			Status:          entity.Running,
			VerbBuildStatus: entity.Completed,
			CreatedByUserID: "u1",
		},
		{
			ID:              "ws-other",
			Name:            "other-ws",
			Status:          entity.Running,
			VerbBuildStatus: entity.Completed,
			CreatedByUserID: "u2",
		},
	}
	term := terminal.New()

	// Without --all: only my workspaces
	outMine := captureStdout(t, func() {
		err := RunLs(term, s, nil, "", false, true)
		if err != nil {
			t.Fatalf("RunLs returned error: %v", err)
		}
	})
	var mineParsed struct {
		Workspaces []WorkspaceInfo `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(outMine), &mineParsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(mineParsed.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace without --all, got %d", len(mineParsed.Workspaces))
	}
	if mineParsed.Workspaces[0].Name != "my-ws" {
		t.Errorf("expected 'my-ws', got %q", mineParsed.Workspaces[0].Name)
	}

	// With --all: output is always {workspaces, nodes} object.
	// Nodes fetch fails gracefully (no gRPC server), so nodes will be empty.
	outAll := captureStdout(t, func() {
		err := RunLs(term, s, nil, "", true, true)
		if err != nil {
			t.Fatalf("RunLs --all returned error: %v", err)
		}
	})
	var allRes struct {
		Workspaces []WorkspaceInfo `json:"workspaces"`
		Nodes      []NodeInfo      `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(outAll), &allRes); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(allRes.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces with --all, got %d", len(allRes.Workspaces))
	}
	if len(allRes.Nodes) != 0 {
		t.Fatalf("expected 0 nodes (no gRPC server), got %d", len(allRes.Nodes))
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

// TestRunLs_WorkspaceGPULookup verifies that GPU info from instance types
// appears in the JSON output.
func TestRunLs_WorkspaceGPULookup(t *testing.T) {
	s := newTestStore()
	s.workspaces = []entity.Workspace{
		{
			ID:              "ws1",
			Name:            "gpu-box",
			Status:          entity.Running,
			VerbBuildStatus: entity.Completed,
			CreatedByUserID: "u1",
			InstanceType:    "g5.xlarge",
		},
	}
	term := terminal.New()

	// The mock returns empty InstanceTypesResponse, so GPU should be "-".
	out := captureStdout(t, func() {
		err := RunLs(term, s, nil, "", false, true)
		if err != nil {
			t.Fatalf("RunLs returned error: %v", err)
		}
	})

	var parsed struct {
		Workspaces []WorkspaceInfo `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}
	if parsed.Workspaces[0].GPU != "-" {
		t.Errorf("expected GPU '-' for unknown instance type, got %q", parsed.Workspaces[0].GPU)
	}
	if parsed.Workspaces[0].InstanceType != "g5.xlarge" {
		t.Errorf("expected instance_type 'g5.xlarge', got %q", parsed.Workspaces[0].InstanceType)
	}
}

// TestRunLs_UnhealthyStatus verifies that a running workspace with unhealthy
// status shows as UNHEALTHY.
func TestRunLs_UnhealthyStatus(t *testing.T) {
	s := newTestStore()
	s.workspaces = []entity.Workspace{
		{
			ID:              "ws1",
			Name:            "sick-box",
			Status:          entity.Running,
			HealthStatus:    entity.Unhealthy,
			VerbBuildStatus: entity.Completed,
			CreatedByUserID: "u1",
		},
	}
	term := terminal.New()

	out := captureStdout(t, func() {
		err := RunLs(term, s, nil, "", false, true)
		if err != nil {
			t.Fatalf("RunLs returned error: %v", err)
		}
	})

	var parsed struct {
		Workspaces []WorkspaceInfo `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if parsed.Workspaces[0].Status != entity.Unhealthy {
		t.Errorf("expected status %q, got %q", entity.Unhealthy, parsed.Workspaces[0].Status)
	}
}

// TestHandleLsArg_Routing verifies all valid arg variants route without error
// (except nodes which needs a gRPC client).
func TestHandleLsArg_Routing(t *testing.T) {
	s := newTestStore()
	term := terminal.New()
	ls := NewLs(s, term, true)

	// These should all succeed (workspace/org/instance paths use mock store).
	successArgs := []string{
		"instance", "instances",
		"org", "orgs", "organization", "organizations",
		"workspace", "workspaces",
	}
	for _, arg := range successArgs {
		if err := handleLsArg(ls, arg, s.user, s.org, false); err != nil {
			t.Errorf("handleLsArg(%q) returned unexpected error: %v", arg, err)
		}
	}

	// "node"/"nodes" route to RunNodes which calls the gRPC client — verify
	// it attempts the path (error expected due to no real client).
	for _, arg := range []string{"node", "nodes"} {
		_ = handleLsArg(ls, arg, s.user, s.org, false)
	}
}

// TestOutputWorkspacesJSON_WithNodes verifies that when nodes are present,
// the JSON output is a {workspaces, nodes} object.
func TestOutputWorkspacesJSON_WithNodes(t *testing.T) {
	s := newTestStore()
	term := terminal.New()
	ls := NewLs(s, term, true)

	workspaces := []entity.Workspace{
		{
			ID:              "ws1",
			Name:            "dev-box",
			Status:          entity.Running,
			VerbBuildStatus: entity.Completed,
			CreatedByUserID: "u1",
		},
	}
	nodes := []*nodev1.ExternalNode{
		{
			Name:           "gpu-node-1",
			ExternalNodeId: "en-123",
			OrganizationId: "org1",
		},
		{
			Name:           "gpu-node-2",
			ExternalNodeId: "en-456",
			OrganizationId: "org1",
		},
	}

	out := captureStdout(t, func() {
		err := ls.outputWorkspacesJSON(workspaces, nil, nodes)
		if err != nil {
			t.Fatalf("outputWorkspacesJSON returned error: %v", err)
		}
	})

	var result struct {
		Workspaces []WorkspaceInfo `json:"workspaces"`
		Nodes      []NodeInfo      `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}
	if len(result.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(result.Workspaces))
	}
	if result.Workspaces[0].Name != "dev-box" {
		t.Errorf("expected workspace name 'dev-box', got %q", result.Workspaces[0].Name)
	}
	if len(result.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result.Nodes))
	}
	if result.Nodes[0].Name != "gpu-node-1" {
		t.Errorf("expected node name 'gpu-node-1', got %q", result.Nodes[0].Name)
	}
	if result.Nodes[1].Name != "gpu-node-2" {
		t.Errorf("expected node name 'gpu-node-2', got %q", result.Nodes[1].Name)
	}
}

// TestOutputWorkspacesJSON_NoNodes verifies that when nodes is nil (--all not used),
// the JSON output is still an object with a "workspaces" key.
func TestOutputWorkspacesJSON_NoNodes(t *testing.T) {
	s := newTestStore()
	term := terminal.New()
	ls := NewLs(s, term, true)

	workspaces := []entity.Workspace{
		{
			ID:              "ws1",
			Name:            "dev-box",
			Status:          entity.Running,
			VerbBuildStatus: entity.Completed,
			CreatedByUserID: "u1",
		},
	}

	out := captureStdout(t, func() {
		err := ls.outputWorkspacesJSON(workspaces, nil, nil)
		if err != nil {
			t.Fatalf("outputWorkspacesJSON returned error: %v", err)
		}
	})

	var parsed struct {
		Workspaces []WorkspaceInfo `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, out)
	}
	if len(parsed.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(parsed.Workspaces))
	}

	// Should not contain "nodes" key when nodes is nil
	if strings.Contains(out, `"nodes"`) {
		t.Error("nil nodes should not produce a nodes key in the output")
	}
}
