package gpucreate

import (
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/stretchr/testify/assert"
)

// MockGPUCreateStore is a mock implementation of GPUCreateStore for testing
type MockGPUCreateStore struct {
	User                *entity.User
	Org                 *entity.Organization
	Workspaces          map[string]*entity.Workspace
	CreateError         error
	CreateErrorTypes    map[string]error // Errors for specific instance types
	DeleteError         error
	CreatedWorkspaces   []*entity.Workspace
	DeletedWorkspaceIDs []string
}

func NewMockGPUCreateStore() *MockGPUCreateStore {
	return &MockGPUCreateStore{
		User: &entity.User{
			ID:             "user-123",
			GlobalUserType: "Standard",
		},
		Org: &entity.Organization{
			ID:   "org-123",
			Name: "test-org",
		},
		Workspaces:          make(map[string]*entity.Workspace),
		CreateErrorTypes:    make(map[string]error),
		CreatedWorkspaces:   []*entity.Workspace{},
		DeletedWorkspaceIDs: []string{},
	}
}

func (m *MockGPUCreateStore) GetCurrentUser() (*entity.User, error) {
	return m.User, nil
}

func (m *MockGPUCreateStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return m.Org, nil
}

func (m *MockGPUCreateStore) GetWorkspace(workspaceID string) (*entity.Workspace, error) {
	if ws, ok := m.Workspaces[workspaceID]; ok {
		return ws, nil
	}
	return &entity.Workspace{
		ID:     workspaceID,
		Status: entity.Running,
	}, nil
}

func (m *MockGPUCreateStore) CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error) {
	// Check for type-specific errors first
	if err, ok := m.CreateErrorTypes[options.InstanceType]; ok {
		return nil, err
	}

	if m.CreateError != nil {
		return nil, m.CreateError
	}

	ws := &entity.Workspace{
		ID:           "ws-" + options.Name,
		Name:         options.Name,
		InstanceType: options.InstanceType,
		Status:       entity.Running,
	}
	m.Workspaces[ws.ID] = ws
	m.CreatedWorkspaces = append(m.CreatedWorkspaces, ws)
	return ws, nil
}

func (m *MockGPUCreateStore) DeleteWorkspace(workspaceID string) (*entity.Workspace, error) {
	if m.DeleteError != nil {
		return nil, m.DeleteError
	}

	m.DeletedWorkspaceIDs = append(m.DeletedWorkspaceIDs, workspaceID)
	ws := m.Workspaces[workspaceID]
	delete(m.Workspaces, workspaceID)
	return ws, nil
}

func (m *MockGPUCreateStore) GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error) {
	return []entity.Workspace{}, nil
}

func (m *MockGPUCreateStore) GetAllInstanceTypesWithWorkspaceGroups(orgID string) (*gpusearch.AllInstanceTypesResponse, error) {
	return nil, nil
}

func (m *MockGPUCreateStore) GetInstanceTypes() (*gpusearch.InstanceTypesResponse, error) {
	// Return a default set of instance types for testing
	return &gpusearch.InstanceTypesResponse{
		Items: []gpusearch.InstanceType{
			{
				Type: "g5.xlarge",
				SupportedGPUs: []gpusearch.GPU{
					{Count: 1, Name: "A10G", Manufacturer: "NVIDIA", Memory: "24GiB"},
				},
				SupportedStorage: []gpusearch.Storage{
					{Size: "500GiB"},
				},
				Memory:              "16GiB",
				VCPU:                4,
				BasePrice:           gpusearch.BasePrice{Currency: "USD", Amount: "1.006"},
				EstimatedDeployTime: "5m0s",
			},
		},
	}, nil
}

func TestIsValidInstanceType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid AWS instance type", "g5.xlarge", true},
		{"Valid AWS large instance", "p4d.24xlarge", true},
		{"Valid GCP instance type", "n1-highmem-4:nvidia-tesla-t4:1", true},
		{"Single letter", "a", false},
		{"No numbers", "xlarge", false},
		{"No letters", "12345", false},
		{"Empty string", "", false},
		{"Single character", "1", false},
		{"Valid with colon", "g5:xlarge", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidInstanceType(tt.input)
			assert.Equal(t, tt.expected, result, "Validation failed for %s", tt.input)
		})
	}
}

func TestParseInstanceTypesFromFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Single type", "g5.xlarge", []string{"g5.xlarge"}},
		{"Multiple types comma separated", "g5.xlarge,g5.2xlarge,p3.2xlarge", []string{"g5.xlarge", "g5.2xlarge", "p3.2xlarge"}},
		{"With spaces", "g5.xlarge, g5.2xlarge, p3.2xlarge", []string{"g5.xlarge", "g5.2xlarge", "p3.2xlarge"}},
		{"Empty string", "", []string{}},
		{"Only spaces", "   ", []string{}},
		{"Trailing comma", "g5.xlarge,", []string{"g5.xlarge"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseInstanceTypes(tt.input)
			assert.NoError(t, err)

			// Handle nil vs empty slice
			if len(tt.expected) == 0 {
				assert.Empty(t, result)
			} else {
				// Compare just the Type field of each InstanceSpec
				var resultTypes []string
				for _, spec := range result {
					resultTypes = append(resultTypes, spec.Type)
				}
				assert.Equal(t, tt.expected, resultTypes)
			}
		})
	}
}

func TestGPUCreateOptions(t *testing.T) {
	opts := GPUCreateOptions{
		Name: "my-instance",
		InstanceTypes: []InstanceSpec{
			{Type: "g5.xlarge", DiskGB: 500},
			{Type: "g5.2xlarge"},
		},
		Count:    2,
		Parallel: 3,
		Detached: true,
	}

	assert.Equal(t, "my-instance", opts.Name)
	assert.Len(t, opts.InstanceTypes, 2)
	assert.Equal(t, "g5.xlarge", opts.InstanceTypes[0].Type)
	assert.Equal(t, 500.0, opts.InstanceTypes[0].DiskGB)
	assert.Equal(t, "g5.2xlarge", opts.InstanceTypes[1].Type)
	assert.Equal(t, 0.0, opts.InstanceTypes[1].DiskGB)
	assert.Equal(t, 2, opts.Count)
	assert.Equal(t, 3, opts.Parallel)
	assert.True(t, opts.Detached)
}

func TestResolveWorkspaceUserOptionsStandard(t *testing.T) {
	user := &entity.User{
		ID:             "user-123",
		GlobalUserType: "Standard",
	}

	options := &store.CreateWorkspacesOptions{}
	result := resolveWorkspaceUserOptions(options, user)

	assert.Equal(t, store.UserWorkspaceTemplateID, result.WorkspaceTemplateID)
	assert.Equal(t, store.UserWorkspaceClassID, result.WorkspaceClassID)
}

func TestResolveWorkspaceUserOptionsAdmin(t *testing.T) {
	user := &entity.User{
		ID:             "user-123",
		GlobalUserType: "Admin",
	}

	options := &store.CreateWorkspacesOptions{}
	result := resolveWorkspaceUserOptions(options, user)

	assert.Equal(t, store.DevWorkspaceTemplateID, result.WorkspaceTemplateID)
	assert.Equal(t, store.DevWorkspaceClassID, result.WorkspaceClassID)
}

func TestResolveWorkspaceUserOptionsPreserveExisting(t *testing.T) {
	user := &entity.User{
		ID:             "user-123",
		GlobalUserType: "Standard",
	}

	options := &store.CreateWorkspacesOptions{
		WorkspaceTemplateID: "custom-template",
		WorkspaceClassID:    "custom-class",
	}
	result := resolveWorkspaceUserOptions(options, user)

	// Should preserve existing values
	assert.Equal(t, "custom-template", result.WorkspaceTemplateID)
	assert.Equal(t, "custom-class", result.WorkspaceClassID)
}

func TestMockGPUCreateStoreBasics(t *testing.T) {
	mock := NewMockGPUCreateStore()

	user, err := mock.GetCurrentUser()
	assert.NoError(t, err)
	assert.Equal(t, "user-123", user.ID)

	org, err := mock.GetActiveOrganizationOrDefault()
	assert.NoError(t, err)
	assert.Equal(t, "org-123", org.ID)
}

func TestMockGPUCreateStoreCreateWorkspace(t *testing.T) {
	mock := NewMockGPUCreateStore()

	options := store.NewCreateWorkspacesOptions("cluster-1", "test-instance")
	options.WithInstanceType("g5.xlarge")

	ws, err := mock.CreateWorkspace("org-123", options)
	assert.NoError(t, err)
	assert.Equal(t, "test-instance", ws.Name)
	assert.Equal(t, "g5.xlarge", ws.InstanceType)
	assert.Len(t, mock.CreatedWorkspaces, 1)
}

func TestMockGPUCreateStoreDeleteWorkspace(t *testing.T) {
	mock := NewMockGPUCreateStore()

	// First create a workspace
	options := store.NewCreateWorkspacesOptions("cluster-1", "test-instance")
	ws, _ := mock.CreateWorkspace("org-123", options)

	// Then delete it
	_, err := mock.DeleteWorkspace(ws.ID)
	assert.NoError(t, err)
	assert.Contains(t, mock.DeletedWorkspaceIDs, ws.ID)
}

func TestMockGPUCreateStoreTypeSpecificError(t *testing.T) {
	mock := NewMockGPUCreateStore()
	mock.CreateErrorTypes["g5.xlarge"] = assert.AnError

	options := store.NewCreateWorkspacesOptions("cluster-1", "test-instance")
	options.WithInstanceType("g5.xlarge")

	_, err := mock.CreateWorkspace("org-123", options)
	assert.Error(t, err)

	// Different type should work
	options2 := store.NewCreateWorkspacesOptions("cluster-1", "test-instance-2")
	options2.WithInstanceType("g5.2xlarge")

	ws, err := mock.CreateWorkspace("org-123", options2)
	assert.NoError(t, err)
	assert.NotNil(t, ws)
}

func TestGetFilteredInstanceTypesDefaults(t *testing.T) {
	mock := NewMockGPUCreateStore()

	// Get instance types with no user filters (uses defaults):
	// - 24GB VRAM (>= 20GB total VRAM requirement)
	// - 500GB disk (>= 500GB requirement)
	// - A10G GPU = 8.6 capability (>= 8.0 requirement)
	// - 5m boot time (< 7m requirement)
	specs, err := getFilteredInstanceTypes(mock, &searchFilterFlags{})
	assert.NoError(t, err)
	assert.Len(t, specs, 1)
	assert.Equal(t, "g5.xlarge", specs[0].Type)
	assert.Equal(t, 500.0, specs[0].DiskGB) // Should use the instance's disk size
}

func TestGetFilteredInstanceTypesWithGPUName(t *testing.T) {
	mock := NewMockGPUCreateStore()

	// Filter by GPU name that matches the mock data
	specs, err := getFilteredInstanceTypes(mock, &searchFilterFlags{gpuName: "A10G"})
	assert.NoError(t, err)
	assert.Len(t, specs, 1)
	assert.Equal(t, "g5.xlarge", specs[0].Type)
}

func TestGetFilteredInstanceTypesNoMatch(t *testing.T) {
	mock := NewMockGPUCreateStore()

	// Filter by GPU name that doesn't match
	specs, err := getFilteredInstanceTypes(mock, &searchFilterFlags{gpuName: "H100"})
	assert.NoError(t, err)
	assert.Len(t, specs, 0)
}

func TestParseTableInput(t *testing.T) {
	tableInput := strings.Join([]string{
		"TYPE           TARGET_DISK  GPU    COUNT  VRAM/GPU  TOTAL VRAM  CAPABILITY  VCPUs  $/HR",
		"g5.xlarge      500          A10G   1      24 GB     24 GB       8.6         4      $1.01",
		"g5.2xlarge     500          A10G   1      24 GB     24 GB       8.6         8      $1.21",
		"p4d.24xlarge   1000         A100   8      40 GB     320 GB      8.0         96     $32.77",
		"",
		"Found 3 GPU instance types",
	}, "\n")

	specs := parseTableInput(tableInput)

	assert.Len(t, specs, 3)
	assert.Equal(t, "g5.xlarge", specs[0].Type)
	assert.Equal(t, 500.0, specs[0].DiskGB)
	assert.Equal(t, "g5.2xlarge", specs[1].Type)
	assert.Equal(t, 500.0, specs[1].DiskGB)
	assert.Equal(t, "p4d.24xlarge", specs[2].Type)
	assert.Equal(t, 1000.0, specs[2].DiskGB)
}

func TestParseJSONInput(t *testing.T) {
	// Simulated JSON output from gpu-search --json
	jsonInput := `[
		{
			"type": "g5.xlarge",
			"provider": "aws",
			"gpu_name": "A10G",
			"target_disk_gb": 1000
		},
		{
			"type": "p4d.24xlarge",
			"provider": "aws",
			"gpu_name": "A100",
			"target_disk_gb": 500
		},
		{
			"type": "g6.xlarge",
			"provider": "aws",
			"gpu_name": "L4"
		}
	]`

	specs, err := parseJSONInput(jsonInput)
	assert.NoError(t, err)
	assert.Len(t, specs, 3)

	// Check first instance with disk
	assert.Equal(t, "g5.xlarge", specs[0].Type)
	assert.Equal(t, 1000.0, specs[0].DiskGB)

	// Check second instance with different disk
	assert.Equal(t, "p4d.24xlarge", specs[1].Type)
	assert.Equal(t, 500.0, specs[1].DiskGB)

	// Check third instance without disk (should be 0)
	assert.Equal(t, "g6.xlarge", specs[2].Type)
	assert.Equal(t, 0.0, specs[2].DiskGB)
}

func TestFormatInstanceSpecs(t *testing.T) {
	specs := []InstanceSpec{
		{Type: "g5.xlarge", DiskGB: 1000},
		{Type: "p4d.24xlarge", DiskGB: 0},
		{Type: "g6.xlarge", DiskGB: 500},
	}

	result := formatInstanceSpecs(specs)
	assert.Equal(t, "g5.xlarge (1000GB disk), p4d.24xlarge, g6.xlarge (500GB disk)", result)
}
