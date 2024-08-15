package shell

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockShellStore struct {
	mock.Mock
	store.FileStore
}

func (m *MockShellStore) GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error) {
	args := m.Called(options)
	return args.Get(0).([]entity.Organization), args.Error(1)
}

func (m *MockShellStore) GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	args := m.Called(organizationID, options)
	return args.Get(0).([]entity.Workspace), args.Error(1)
}

func (m *MockShellStore) StartWorkspace(workspaceID string) (*entity.Workspace, error) {
	args := m.Called(workspaceID)
	return args.Get(0).(*entity.Workspace), args.Error(1)
}

func (m *MockShellStore) GetWorkspace(workspaceID string) (*entity.Workspace, error) {
	args := m.Called(workspaceID)
	return args.Get(0).(*entity.Workspace), args.Error(1)
}

func (m *MockShellStore) GetCurrentUserKeys() (*entity.UserKeys, error) {
	args := m.Called()
	return args.Get(0).(*entity.UserKeys), args.Error(1)
}

func (m *MockShellStore) GetCurrentUser() (*entity.User, error) {
	args := m.Called()
	return args.Get(0).(*entity.User), args.Error(1)
}

func (m *MockShellStore) GetWorkspaceByNameOrIDErr(orgID, workspaceNameOrID string) (*entity.Workspace, error) {
	args := m.Called(orgID, workspaceNameOrID)
	return args.Get(0).(*entity.Workspace), args.Error(1)
}

func (m *MockShellStore) CopyBin(targetBin string) error {
	args := m.Called(targetBin)
	return args.Error(0)
}

func (m *MockShellStore) DoesJetbrainsFilePathExist() (bool, error) {
	args := m.Called()
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockShellStore) DownloadBinary(url string, target string) error {
	args := m.Called(url, target)
	return args.Error(0)
}

func (m *MockShellStore) FileExists(filepath string) (bool, error) {
	args := m.Called(filepath)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockShellStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	args := m.Called()
	return args.Get(0).(*entity.Organization), args.Error(1)
}

func (m *MockShellStore) GetBrevSSHConfigPath() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) GetContextWorkspaces() ([]entity.Workspace, error) {
	args := m.Called()
	return args.Get(0).([]entity.Workspace), args.Error(1)
}

func (m *MockShellStore) WriteBrevSSHConfig(config string) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockShellStore) GetUserSSHConfig() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) WriteUserSSHConfig(config string) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockShellStore) GetPrivateKeyPath() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) GetUserSSHConfigPath() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) GetJetBrainsConfigPath() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) GetJetBrainsConfig() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) WriteJetBrainsConfig(config string) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockShellStore) GetWSLHostUserSSHConfigPath() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) GetWindowsDir() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) WriteBrevSSHConfigWSL(config string) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockShellStore) GetFileAsString(path string) (string, error) {
	args := m.Called(path)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) GetWSLHostBrevSSHConfigPath() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) GetWSLUserSSHConfig() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockShellStore) WriteWSLUserSSHConfig(config string) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockShellStore) GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error) {
	args := m.Called(orgID, nameOrID)
	return args.Get(0).([]entity.Workspace), args.Error(1)
}

func TestNewCmdShell_SSHClientNotInstalled(t *testing.T) {
	mockTerminal, outBuffer, _, errBuffer := terminal.NewTestTerminal()
	mockStore := new(MockShellStore)

	cmd := newCmdShellWithDeps(mockTerminal, mockStore, mockStore, func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	})

	cmd.SetOut(outBuffer)
	cmd.SetErr(errBuffer)
	cmd.SetArgs([]string{"workspace-1"})

	// Run the command
	err := cmd.Execute()

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSH client is not installed or not available in PATH")
}

func TestNewCmdShell_SSHClientWorkspaceNotFound(t *testing.T) {
	mockTerminal, outBuffer, _, errBuffer := terminal.NewTestTerminal()
	mockStore := new(MockShellStore)

	testUser := &entity.User{
		ID:   "test-user-id",
		Name: "Test User",
	}
	testOrg := &entity.Organization{
		ID:   "test-org-id",
		Name: "test-org",
	}
	testWorkspaces := []entity.Workspace{
		{
			ID:              "workspace-1",
			Name:            "Workspace 1",
			CreatedByUserID: "test-user-id",
			Status:          entity.Running,
		},
	}
	mockStore.On("GetCurrentUser").Return(testUser, nil)
	mockStore.On("GetActiveOrganizationOrDefault").Return(testOrg, nil)
	mockStore.On("GetWorkspaceByNameOrID", testOrg.ID, "workspace-1").Return(testWorkspaces, nil)
	mockStore.On("GetWorkspace", "workspace-1").Return((*entity.Workspace)(nil), errors.New("workspace not found"))

	cmd := NewCmdShell(mockTerminal, mockStore, mockStore)

	cmd.SetOut(outBuffer)
	cmd.SetErr(errBuffer)
	cmd.SetArgs([]string{"workspace-1"})

	err := cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, errBuffer.String(), "waiting for instance to be ready")
	assert.Contains(t, errBuffer.String(), "workspace not found")
}
