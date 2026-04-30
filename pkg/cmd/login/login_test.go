package login

import (
	"bytes"
	"testing"

	authpkg "github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLoginAuth struct {
	apiKeyCalls int
	apiKey      string
	tokenCalls  int
	loginCalls  int
}

func (m *mockLoginAuth) Login(_ bool) (*authpkg.LoginTokens, error) {
	m.loginCalls++
	return &authpkg.LoginTokens{}, nil
}

func (m *mockLoginAuth) LoginWithToken(_ string) error {
	m.tokenCalls++
	return nil
}

func (m *mockLoginAuth) LoginWithAPIKey(apiKey string) error {
	m.apiKeyCalls++
	m.apiKey = apiKey
	return nil
}

type mockLoginStore struct {
	getCurrentUserCalls     int
	createUserCalls         int
	getOrCreateOrgCalls     int
	createOrganizationCalls int
	setDefaultOrgCalls      int
	updateUserCalls         int
	userHomeDirCalls        int
	defaultOrg              *entity.Organization
}

func (m *mockLoginStore) SaveAuthTokens(_ entity.AuthTokens) error   { return nil }
func (m *mockLoginStore) GetAuthTokens() (*entity.AuthTokens, error) { return nil, nil }
func (m *mockLoginStore) DeleteAuthTokens() error                    { return nil }

func (m *mockLoginStore) GetCurrentUser() (*entity.User, error) {
	m.getCurrentUserCalls++
	return &entity.User{ID: "user-1", Username: "testuser", Name: "Test User", Email: "test@example.com"}, nil
}

func (m *mockLoginStore) CreateUser(_ string) (*entity.User, error) {
	m.createUserCalls++
	return &entity.User{}, nil
}

func (m *mockLoginStore) SetDefaultOrganization(org *entity.Organization) error {
	m.setDefaultOrgCalls++
	m.defaultOrg = org
	return nil
}

func (m *mockLoginStore) GetOrganizations(_ *store.GetOrganizationsOptions) ([]entity.Organization, error) {
	return []entity.Organization{{ID: "org-1", Name: "org"}}, nil
}

func (m *mockLoginStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	m.getOrCreateOrgCalls++
	return &entity.Organization{ID: "org-1", Name: "org"}, nil
}

func (m *mockLoginStore) CreateOrganization(_ store.CreateOrganizationRequest) (*entity.Organization, error) {
	m.createOrganizationCalls++
	return &entity.Organization{ID: "org-1", Name: "org"}, nil
}

func (m *mockLoginStore) GetServerSockFile() string { return "" }

func (m *mockLoginStore) GetWorkspaces(_ string, _ *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	return nil, nil
}

func (m *mockLoginStore) UpdateUser(_ string, updatedUser *entity.UpdateUser) (*entity.User, error) {
	m.updateUserCalls++
	return &entity.User{
		ID:             "user-1",
		Username:       updatedUser.Username,
		Name:           updatedUser.Name,
		Email:          updatedUser.Email,
		OnboardingData: updatedUser.OnboardingData,
	}, nil
}

func (m *mockLoginStore) UserHomeDir() (string, error) {
	m.userHomeDirCalls++
	return "/home/testuser", nil
}

func (m *mockLoginStore) GetAllWorkspaces(_ *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	return nil, nil
}
func (m *mockLoginStore) GetCurrentWorkspaceID() (string, error) { return "", nil }
func (m *mockLoginStore) GetWindowsDir() (string, error)         { return "", nil }

func TestRunLoginWithAPIKey_SavesKeyAndOrgWithoutUserOrBackendOrgCalls(t *testing.T) {
	auth := &mockLoginAuth{}
	loginStore := &mockLoginStore{}
	opts := LoginOptions{Auth: auth, LoginStore: loginStore}

	err := opts.RunLogin(terminal.New(), "", "  brev_api_test-key  ", "  org-test  ", false, "", "")

	require.NoError(t, err)
	assert.Equal(t, 1, auth.apiKeyCalls)
	assert.Equal(t, "brev_api_test-key", auth.apiKey)
	assert.Equal(t, 1, loginStore.setDefaultOrgCalls)
	require.NotNil(t, loginStore.defaultOrg)
	assert.Equal(t, "org-test", loginStore.defaultOrg.ID)
	assert.Equal(t, "org-test", loginStore.defaultOrg.Name)
	assert.Equal(t, 0, auth.tokenCalls)
	assert.Equal(t, 0, auth.loginCalls)
	assert.Equal(t, 0, loginStore.getCurrentUserCalls)
	assert.Equal(t, 0, loginStore.createUserCalls)
	assert.Equal(t, 0, loginStore.getOrCreateOrgCalls)
	assert.Equal(t, 0, loginStore.createOrganizationCalls)
	assert.Equal(t, 0, loginStore.updateUserCalls)
}

func TestRunLoginWithAPIKey_RejectsConflictingFlags(t *testing.T) {
	tests := []struct {
		name             string
		loginToken       string
		skipBrowser      bool
		emailFlag        string
		authProviderFlag string
	}{
		{name: "token", loginToken: "token"},
		{name: "skip browser", skipBrowser: true},
		{name: "email", emailFlag: "user@example.com"},
		{name: "auth provider", authProviderFlag: "nvidia"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &mockLoginAuth{}
			opts := LoginOptions{Auth: auth, LoginStore: &mockLoginStore{}}

			err := opts.RunLogin(terminal.New(), tt.loginToken, "brev_api_test-key", "org-test", tt.skipBrowser, tt.emailFlag, tt.authProviderFlag)

			require.Error(t, err)
			assert.Equal(t, 0, auth.apiKeyCalls)
		})
	}
}

func TestNewCmdLoginWithAPIKey_SkipsPostLoginHooks(t *testing.T) {
	auth := &mockLoginAuth{}
	loginStore := &mockLoginStore{}
	cmd := NewCmdLogin(terminal.New(), loginStore, auth)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--api-key", "brev_api_test-key", "--org-id", "org-test"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, 1, auth.apiKeyCalls)
	assert.Equal(t, 1, loginStore.setDefaultOrgCalls)
	assert.Equal(t, 0, loginStore.getCurrentUserCalls)
	assert.Equal(t, 0, loginStore.updateUserCalls)
	assert.Equal(t, 0, loginStore.userHomeDirCalls)
}

func TestNewCmdLogin_HidesAPIKeyFlagsFromHelp(t *testing.T) {
	cmd := NewCmdLogin(terminal.New(), &mockLoginStore{}, &mockLoginAuth{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.NotContains(t, out.String(), "--api-key")
	assert.NotContains(t, out.String(), "--org-id")
}

func TestRunLoginWithAPIKey_RejectsMissingOrgID(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		orgID  string
	}{
		{name: "missing org id", apiKey: "brev_api_test-key", orgID: "  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &mockLoginAuth{}
			loginStore := &mockLoginStore{}
			opts := LoginOptions{Auth: auth, LoginStore: loginStore}

			err := opts.RunLogin(terminal.New(), "", tt.apiKey, tt.orgID, false, "", "")

			require.Error(t, err)
			assert.Equal(t, 0, auth.apiKeyCalls)
			assert.Equal(t, 0, loginStore.setDefaultOrgCalls)
		})
	}
}

func TestRunLoginWithOrgIDWithoutAPIKeyRejects(t *testing.T) {
	auth := &mockLoginAuth{}
	loginStore := &mockLoginStore{}
	opts := LoginOptions{Auth: auth, LoginStore: loginStore}

	err := opts.RunLogin(terminal.New(), "", "", "org-test", false, "", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "org-id can only be used with api-key")
	assert.Equal(t, 0, auth.apiKeyCalls)
	assert.Equal(t, 0, loginStore.setDefaultOrgCalls)
}
