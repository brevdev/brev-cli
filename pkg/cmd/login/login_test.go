package login

import (
	"bytes"
	"os"
	"testing"

	authpkg "github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIKey = authpkg.BrevAPIKeyPrefix + "test-key"

type mockLoginAuth struct {
	apiKeyCalls int
	apiKey      string
	apiKeyOrgID string
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

func (m *mockLoginAuth) LoginWithAPIKey(apiKey string, orgID string) error {
	m.apiKeyCalls++
	m.apiKey = apiKey
	m.apiKeyOrgID = orgID
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
	listOrgs                []entity.Organization
	listOrgsErr             error
	listOrgsFn              func() ([]entity.Organization, error)
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

func (m *mockLoginStore) ListOrganizations() ([]entity.Organization, error) {
	if m.listOrgsFn != nil {
		return m.listOrgsFn()
	}
	return m.listOrgs, m.listOrgsErr
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

func TestRunLoginWithAPIKey_SavesKeyAndResolvedOrg(t *testing.T) {
	auth := &mockLoginAuth{}
	loginStore := &mockLoginStore{listOrgs: []entity.Organization{{ID: "org-test", Name: "TestOrg"}}}
	opts := LoginOptions{Auth: auth, LoginStore: loginStore}

	err := opts.RunLogin(terminal.New(), "", "  "+testAPIKey+"  ", "", false, "", "")

	require.NoError(t, err)
	assert.Equal(t, 1, auth.apiKeyCalls)
	assert.Equal(t, testAPIKey, auth.apiKey)
	assert.Equal(t, "org-test", auth.apiKeyOrgID)
	assert.Equal(t, 1, loginStore.setDefaultOrgCalls)
	require.NotNil(t, loginStore.defaultOrg)
	assert.Equal(t, "org-test", loginStore.defaultOrg.ID)
	assert.Equal(t, "TestOrg", loginStore.defaultOrg.Name)
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

			err := opts.RunLogin(terminal.New(), tt.loginToken, testAPIKey, "org-test", tt.skipBrowser, tt.emailFlag, tt.authProviderFlag)

			require.Error(t, err)
			assert.Equal(t, 0, auth.apiKeyCalls)
		})
	}
}

func TestNewCmdLoginWithAPIKey_SkipsPostLoginHooks(t *testing.T) {
	auth := &mockLoginAuth{}
	loginStore := &mockLoginStore{listOrgs: []entity.Organization{{ID: "org-test", Name: "TestOrg"}}}
	cmd := NewCmdLogin(terminal.New(), loginStore, auth)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--api-key", testAPIKey, "--org-id", "org-test"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, 1, auth.apiKeyCalls)
	assert.Equal(t, 1, loginStore.setDefaultOrgCalls)
	assert.Equal(t, 0, loginStore.getCurrentUserCalls)
	assert.Equal(t, 0, loginStore.updateUserCalls)
	assert.Equal(t, 0, loginStore.userHomeDirCalls)
}

func TestNewCmdLogin_OrgIDFlagDeprecationWarning(t *testing.T) {
	auth := &mockLoginAuth{}
	loginStore := &mockLoginStore{listOrgs: []entity.Organization{{ID: "org-test", Name: "TestOrg"}}}
	cmd := NewCmdLogin(terminal.New(), loginStore, auth)
	var out bytes.Buffer // cobra prints deprecated-flag warnings via c.Print -> OutOrStderr (stdout)
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--api-key", testAPIKey, "--org-id", "org-test"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, out.String(), "--org-id has been deprecated", "passing --org-id should warn")
	assert.Contains(t, out.String(), "resolved automatically from the API key")
}

func TestNewCmdLogin_HidesAPIKeyFlagsFromHelp(t *testing.T) {
	cmd := NewCmdLogin(terminal.New(), &mockLoginStore{listOrgs: []entity.Organization{{ID: "org-test", Name: "TestOrg"}}}, &mockLoginAuth{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.NotContains(t, out.String(), "--api-key")
	assert.NotContains(t, out.String(), "--org-id")
}

func TestRunLoginWithAPIKey_AutoResolvesOrgWhenOrgIDOmitted(t *testing.T) {
	auth := &mockLoginAuth{}
	loginStore := &mockLoginStore{listOrgs: []entity.Organization{{ID: "org-123", Name: "TestOrg"}}}
	opts := LoginOptions{Auth: auth, LoginStore: loginStore}

	err := opts.RunLogin(terminal.New(), "", testAPIKey, "", false, "", "")

	require.NoError(t, err)
	assert.Equal(t, 1, auth.apiKeyCalls)
	assert.Equal(t, "org-123", auth.apiKeyOrgID, "resolved org ID should be saved")
	require.NotNil(t, loginStore.defaultOrg)
	assert.Equal(t, "org-123", loginStore.defaultOrg.ID)
}

func TestRunLoginWithAPIKey_ResolveOrgFailureRejects(t *testing.T) {
	auth := &mockLoginAuth{}
	loginStore := &mockLoginStore{listOrgsErr: assert.AnError}
	opts := LoginOptions{Auth: auth, LoginStore: loginStore}

	err := opts.RunLogin(terminal.New(), "", testAPIKey, "", false, "", "")

	require.Error(t, err)
	assert.Equal(t, 0, auth.apiKeyCalls, "must not save when the key can't be resolved/validated")
	assert.Equal(t, 0, loginStore.setDefaultOrgCalls)
}

// 'BREV_ACCESS_KEY=key-A brev login --api-key key-B' must resolve key-B's org: the
// flag key is promoted to the env var so the ListOrganizations call authenticates
// with it, not the pre-existing env key.
func TestRunLoginWithAPIKey_FlagKeyActivatesOverEnvKey(t *testing.T) {
	t.Setenv(authpkg.AccessKeyEnvVar, authpkg.BrevAPIKeyPrefix+"env-key")
	auth := &mockLoginAuth{}
	var seenEnv string
	loginStore := &mockLoginStore{}
	loginStore.listOrgsErr = nil
	loginStore.listOrgs = []entity.Organization{{ID: "org-flag", Name: "FlagOrg"}}
	// Capture the env at ListOrganizations time to prove the flag key is active.
	loginStore.listOrgsFn = func() ([]entity.Organization, error) {
		seenEnv = os.Getenv(authpkg.AccessKeyEnvVar)
		return loginStore.listOrgs, nil
	}
	opts := LoginOptions{Auth: auth, LoginStore: loginStore}

	err := opts.RunLogin(terminal.New(), "", testAPIKey, "", false, "", "")

	require.NoError(t, err)
	assert.Equal(t, testAPIKey, seenEnv, "flag key must be active during org resolution")
	assert.Equal(t, testAPIKey, auth.apiKey, "flag key must be persisted")
	assert.Equal(t, "org-flag", auth.apiKeyOrgID, "flag key's org must be saved")
}

// Browser/token login is an explicit "log me in as this user" action and must
// suppress BREV_ACCESS_KEY for the whole transaction; otherwise post-login calls
// (org selection, breadcrumbs) authenticate with the env key, not the saved JWT.
func TestRunLogin_TokenLoginSuppressesEnvAccessKey(t *testing.T) {
	t.Setenv(authpkg.AccessKeyEnvVar, authpkg.BrevAPIKeyPrefix+"env-key")
	auth := &mockLoginAuth{}
	opts := LoginOptions{Auth: auth, LoginStore: &mockLoginStore{}}

	err := opts.RunLogin(terminal.New(), "some-login-token", "", "", false, "", "")

	require.NoError(t, err)
	assert.Equal(t, "", os.Getenv(authpkg.AccessKeyEnvVar), "browser/token login must clear BREV_ACCESS_KEY so the saved JWT is used")
	assert.Equal(t, 0, auth.apiKeyCalls, "token login must not take the --api-key path")
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
