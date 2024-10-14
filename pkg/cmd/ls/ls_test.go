package ls

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLsStore is a mock implementation of the LsStore interface
type MockLsStore struct {
	mock.Mock
}

func (m *MockLsStore) UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error) {
	args := m.Called(userID, updatedUser)
	return args.Get(0).(*entity.User), args.Error(1)
}

func (m *MockLsStore) GetCurrentWorkspaceID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockLsStore) GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	args := m.Called()
	return args.Get(0).([]entity.Workspace), args.Error(1)
}

func (m *MockLsStore) GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	args := m.Called(organizationID, options)
	return args.Get(0).([]entity.Workspace), args.Error(1)
}

func (m *MockLsStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	args := m.Called()
	return args.Get(0).(*entity.Organization), args.Error(1)
}

func (m *MockLsStore) GetCurrentUser() (*entity.User, error) {
	args := m.Called()
	return args.Get(0).(*entity.User), args.Error(1)
}

func (m *MockLsStore) GetUsers(queryParams map[string]string) ([]entity.User, error) {
	args := m.Called(queryParams)
	return args.Get(0).([]entity.User), args.Error(1)
}

func (m *MockLsStore) GetWorkspace(workspaceID string) (*entity.Workspace, error) {
	args := m.Called(workspaceID)
	return args.Get(0).(*entity.Workspace), args.Error(1)
}

func (m *MockLsStore) GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error) {
	args := m.Called(options)
	return args.Get(0).([]entity.Organization), args.Error(1)
}

func TestLsCommand_RunLs_NoArgs(t *testing.T) {
	mockTerminal, outBuffer, verboseBuffer, errBuffer := terminal.NewTestTerminal()

	mockLsStore := new(MockLsStore)
	testOrg := &entity.Organization{
		ID:   "test-org-id",
		Name: "test-org",
	}
	testUser := &entity.User{
		ID:   "test-user-id",
		Name: "Test User",
	}
	testWorkspaces := []entity.Workspace{
		{
			ID:              "workspace-1",
			Name:            "Workspace 1",
			CreatedByUserID: "test-user-id",
			Status:          entity.Running,
		},
	}

	mockLsStore.On("GetCurrentUser").Return(testUser, nil)
	mockLsStore.On("GetActiveOrganizationOrDefault").Return(testOrg, nil)
	mockLsStore.On("GetWorkspaces", "test-org-id", (*store.GetWorkspacesOptions)(nil)).Return(testWorkspaces, nil)
	mockLsStore.On("GetOrganizations", (*store.GetOrganizationsOptions)(nil)).Return([]entity.Organization{*testOrg}, nil)
	mockLsStore.On("UpdateUser", testUser.ID, mock.AnythingOfType("*entity.UpdateUser")).Return(testUser, nil)
	mockLsStore.On("GetCurrentWorkspaceID").Return("workspace-1", nil)

	cmd := NewCmdLs(mockTerminal, mockLsStore, mockLsStore)

	cmd.SetOut(outBuffer)
	cmd.SetErr(errBuffer)
	// Execute the command without any arguments
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, verboseBuffer.String(), "You have 1 instances in Org test-org")
	assert.Contains(t, outBuffer.String(), "NAME          STATUS   ID           MACHINE \n Workspace 1   RUNNING  workspace-1   (gpu)  \n")
}

func TestLsCommand_RunLs_OrgArg(t *testing.T) {
	mockTerminal, outBuffer, verboseBuffer, errBuffer := terminal.NewTestTerminal()

	mockLsStore := new(MockLsStore)
	testOrg := &entity.Organization{
		ID:   "test-org-id",
		Name: "test-org",
	}
	testUser := &entity.User{
		ID:   "test-user-id",
		Name: "Test User",
	}

	mockLsStore.On("GetCurrentUser").Return(testUser, nil)
	mockLsStore.On("GetOrganizations", (*store.GetOrganizationsOptions)(nil)).Return([]entity.Organization{*testOrg}, nil)
	mockLsStore.On("UpdateUser", testUser.ID, mock.AnythingOfType("*entity.UpdateUser")).Return(testUser, nil)
	mockLsStore.On("GetCurrentWorkspaceID").Return("workspace-1", nil)
	mockLsStore.On("GetActiveOrganizationOrDefault").Return(testOrg, nil)

	cmd := NewCmdLs(mockTerminal, mockLsStore, mockLsStore)
	cmd.SetOut(outBuffer)
	cmd.SetErr(errBuffer)

	// Execute the command with the "org" argument
	cmd.SetArgs([]string{"org"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, verboseBuffer.String(), "Your organizations:")
	assert.Contains(t, outBuffer.String(), "* test-org")
}

func TestLsCommand_RunLs_InvalidArg(t *testing.T) {
	mockTerminal, outBuffer, _, errBuffer := terminal.NewTestTerminal()

	mockLsStore := new(MockLsStore)
	testOrg := &entity.Organization{
		ID:   "test-org-id",
		Name: "test-org",
	}
	testUser := &entity.User{
		ID:   "test-user-id",
		Name: "Test User",
	}

	mockLsStore.On("GetCurrentUser").Return(testUser, nil)
	mockLsStore.On("GetOrganizations", (*store.GetOrganizationsOptions)(nil)).Return([]entity.Organization{*testOrg}, nil)
	mockLsStore.On("UpdateUser", testUser.ID, mock.AnythingOfType("*entity.UpdateUser")).Return(testUser, nil)
	mockLsStore.On("GetCurrentWorkspaceID").Return("workspace-1", nil)
	mockLsStore.On("GetActiveOrganizationOrDefault").Return(testOrg, nil)

	cmd := NewCmdLs(mockTerminal, mockLsStore, mockLsStore)
	cmd.SetOut(outBuffer)
	cmd.SetErr(errBuffer)

	// Execute the command with the "org" argument
	cmd.SetArgs([]string{"rubbish"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, errBuffer.String(), `invalid argument`)
	assert.Contains(t, outBuffer.String(), "Usage:\n")
}

func TestLsCommand_RunLs_InvalidOrg(t *testing.T) {
	mockTerminal, outBuffer, _, errBuffer := terminal.NewTestTerminal()

	mockLsStore := new(MockLsStore)
	testUser := &entity.User{
		ID:   "test-user-id",
		Name: "Test User",
	}

	mockLsStore.On("GetCurrentUser").Return(testUser, nil)
	mockLsStore.On("GetOrganizations", &store.GetOrganizationsOptions{Name: "invalid-org"}).Return([]entity.Organization{}, nil)
	mockLsStore.On("UpdateUser", testUser.ID, mock.AnythingOfType("*entity.UpdateUser")).Return(testUser, nil)
	mockLsStore.On("GetCurrentWorkspaceID").Return("workspace-1", nil)

	cmd := NewCmdLs(mockTerminal, mockLsStore, mockLsStore)
	cmd.SetOut(outBuffer)
	cmd.SetErr(errBuffer)

	// Execute the command with an invalid --org flag
	cmd.SetArgs([]string{"--org", "invalid-org"})

	err := cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, errBuffer.String(), "no org found with name invalid-org")
}

func TestLsCommand_RunLs_WithOrgFlag(t *testing.T) {
	mockTerminal, outBuffer, verboseBuffer, errBuffer := terminal.NewTestTerminal()
	mockLsStore := new(MockLsStore)
	testOrg := &entity.Organization{
		ID:   "test-org-id",
		Name: "test-org",
	}
	testUser := &entity.User{
		ID:   "test-user-id",
		Name: "Test User",
	}
	testWorkspaces := []entity.Workspace{
		{
			ID:              "workspace-1",
			Name:            "Workspace 1",
			CreatedByUserID: "test-user-id",
			Status:          entity.Running,
		},
	}

	mockLsStore.On("GetCurrentUser").Return(testUser, nil)
	mockLsStore.On("GetOrganizations", (*store.GetOrganizationsOptions)(nil)).Return([]entity.Organization{*testOrg}, nil)
	mockLsStore.On("GetOrganizations", &store.GetOrganizationsOptions{Name: "test-org"}).Return([]entity.Organization{*testOrg}, nil)
	mockLsStore.On("GetWorkspaces", "test-org-id", (*store.GetWorkspacesOptions)(nil)).Return(testWorkspaces, nil)
	mockLsStore.On("UpdateUser", testUser.ID, mock.AnythingOfType("*entity.UpdateUser")).Return(testUser, nil)
	mockLsStore.On("GetCurrentWorkspaceID").Return("workspace-1", nil)

	cmd := NewCmdLs(mockTerminal, mockLsStore, mockLsStore)

	// Execute the command with the --org flag
	cmd.SetOut(outBuffer)
	cmd.SetErr(errBuffer)
	cmd.SetArgs([]string{"--org", "test-org"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, verboseBuffer.String(), "You have 1 instances in Org test-org")
	assert.Contains(t, outBuffer.String(), "Workspace 1")
}

func TestLsCommand_RunLs_AllFlag(t *testing.T) {
	mockTerminal, outBuffer, verboseBuffer, errBuffer := terminal.NewTestTerminal()

	mockLsStore := new(MockLsStore)
	testOrg := &entity.Organization{
		ID:   "test-org-id",
		Name: "test-org",
	}
	testUser := &entity.User{
		ID:   "test-user-id",
		Name: "Test User",
	}
	allWorkspaces := []entity.Workspace{
		{
			ID:              "workspace-1",
			Name:            "Workspace 1",
			CreatedByUserID: "test-user-id",
			Status:          entity.Running,
		},
		{
			ID:              "workspace-2",
			Name:            "Workspace 2",
			CreatedByUserID: "test-user-id-2",
			Status:          entity.Stopped,
		},
	}

	mockLsStore.On("GetCurrentUser").Return(testUser, nil)
	mockLsStore.On("GetActiveOrganizationOrDefault").Return(testOrg, nil)
	mockLsStore.On("GetWorkspaces", "test-org-id", (*store.GetWorkspacesOptions)(nil)).Return(allWorkspaces, nil)
	mockLsStore.On("GetOrganizations", (*store.GetOrganizationsOptions)(nil)).Return([]entity.Organization{*testOrg}, nil)
	mockLsStore.On("UpdateUser", testUser.ID, mock.AnythingOfType("*entity.UpdateUser")).Return(testUser, nil)
	mockLsStore.On("GetCurrentWorkspaceID").Return("workspace-1", nil)

	cmd := NewCmdLs(mockTerminal, mockLsStore, mockLsStore)
	cmd.SetOut(outBuffer)
	cmd.SetErr(errBuffer)
	// Execute the command with the --all flag
	cmd.SetArgs([]string{"--all"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, verboseBuffer.String(), "You have 1 instances in Org test-org\nno other projects in Org test-org\n%!(EXTRA int=0)Invite a teamate:\n\tbrev invite")
	assert.Contains(t, outBuffer.String(), " NAME          STATUS   ID           MACHINE \n Workspace 1   RUNNING  workspace-1   (gpu)  \n")
}
