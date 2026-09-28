package auth

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BrevAPIAuthTestSuite struct {
	suite.Suite
}

func (s *BrevAPIAuthTestSuite) SetupTest() {
}

func TestIsAccessTokenValid(t *testing.T) {
	invalidToken := "blah"
	res, err := isAccessTokenValid(invalidToken)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.False(t, res) {
		return
	}
}

type MockAuthStore struct {
	authTokens *entity.AuthTokens
	saved      entity.AuthTokens
	didSave    bool
}

func (m *MockAuthStore) SaveAuthTokens(tokens entity.AuthTokens) error {
	m.saved = tokens
	m.didSave = true
	m.authTokens = &tokens
	return nil
}

func (m MockAuthStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return m.authTokens, nil
}

func (m MockAuthStore) DeleteAuthTokens() error {
	return nil
}

type MockOauth struct {
	authTokens  *entity.AuthTokens
	loginTokens *LoginTokens
	flowDone    bool
}

func (m *MockOauth) GetCredentialProvider() entity.CredentialProvider {
	return "mock"
}

func (m *MockOauth) IsTokenValid(token string) bool {
	return true
}

func (m *MockOauth) DoDeviceAuthFlow(_ func(string, string)) (*LoginTokens, error) {
	m.flowDone = true
	return m.loginTokens, nil
}

func (m MockOauth) GetNewAuthTokensWithRefresh(_ string) (*entity.AuthTokens, error) {
	return m.authTokens, nil
}

const (
	validToken = "abc"
	testAPIKey = BrevAPIKeyPrefix + "test-key"
)

func TestIsBrevAPIKey(t *testing.T) {
	assert.True(t, IsBrevAPIKey(testAPIKey))
	assert.True(t, IsBrevAPIKey("  "+testAPIKey+"  "))
	assert.False(t, IsBrevAPIKey("bakery-token"))
	assert.False(t, IsBrevAPIKey("jwt-token"))
	assert.False(t, IsBrevAPIKey(""))
}

type sideEffectingTokenStore struct {
	tokens               *entity.AuthTokens
	getAccessTokenCalled bool

	orgs                 []entity.Organization
	listOrganizationsErr error
}

func (s *sideEffectingTokenStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return s.tokens, nil
}

func (s *sideEffectingTokenStore) ListOrganizations() ([]entity.Organization, error) {
	return s.orgs, s.listOrganizationsErr
}

func (s *sideEffectingTokenStore) GetAccessToken() (string, error) {
	s.getAccessTokenCalled = true
	return testAPIKey, nil
}

func TestIsAPIKeyAuthStore_ReadsSavedTokensWithoutAccessTokenSideEffects(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	s := &sideEffectingTokenStore{
		tokens: &entity.AuthTokens{APIKey: testAPIKey},
	}

	assert.True(t, IsAPIKeyAuthStore(s))
	assert.False(t, s.getAccessTokenCalled)
}

func TestIsAPIKeyAuthStore_LegacyCredentialsAreNotAPIKeyAuth(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	s := &sideEffectingTokenStore{
		tokens: &entity.AuthTokens{
			AccessToken:  validToken,
			RefreshToken: "refresh",
		},
	}

	assert.False(t, IsAPIKeyAuthStore(s))
	assert.False(t, s.getAccessTokenCalled)
}

func TestIsAPIKeyAuthStore_EnvKeyIsAPIKeyEvenWhenNotPersisted(t *testing.T) {
	t.Setenv(APIKeyEnvVar, testAPIKey)
	s := &sideEffectingTokenStore{tokens: nil} // nothing persisted
	assert.True(t, IsAPIKeyAuthStore(s))
}

func TestResolveEnvAPIKeyOrg_ResolvesOrgInRealTime(t *testing.T) {
	t.Setenv(APIKeyEnvVar, testAPIKey)
	s := &sideEffectingTokenStore{orgs: []entity.Organization{{ID: "org-realtime", Name: "Realtime Org"}}}
	org, err := ResolveEnvAPIKeyOrg(s)
	assert.NoError(t, err)
	require.NotNil(t, org)
	assert.Equal(t, "org-realtime", org.ID)
	assert.Equal(t, "Realtime Org", org.Name)
}

func TestResolveEnvAPIKeyOrg_NoOrgReturnsError(t *testing.T) {
	t.Setenv(APIKeyEnvVar, testAPIKey)
	s := &sideEffectingTokenStore{}
	_, err := ResolveEnvAPIKeyOrg(s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api key invalid")
}

func TestResolveEnvAPIKeyOrg_MultipleOrgsReturnsError(t *testing.T) {
	t.Setenv(APIKeyEnvVar, testAPIKey)
	s := &sideEffectingTokenStore{orgs: []entity.Organization{
		{ID: "org-1", Name: "One"}, {ID: "org-2", Name: "Two"},
	}}
	_, err := ResolveEnvAPIKeyOrg(s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api key invalid")
}

func TestResolveEnvAPIKeyOrg_ListErrorPropagates(t *testing.T) {
	t.Setenv(APIKeyEnvVar, testAPIKey)
	s := &sideEffectingTokenStore{listOrganizationsErr: errors.New("boom")}
	_, err := ResolveEnvAPIKeyOrg(s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestResolveEnvAPIKeyOrg_NoEnvReturnsNil(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	s := &sideEffectingTokenStore{orgs: []entity.Organization{{ID: "org-realtime", Name: "Realtime Org"}}}
	org, err := ResolveEnvAPIKeyOrg(s)
	assert.NoError(t, err)
	assert.Nil(t, org)
}

// Without an env key, an established API-key login uses the persisted org.
func TestGetAPIKeyOrgID_PersistedOrgReturnsOrg(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	s := &sideEffectingTokenStore{tokens: &entity.AuthTokens{
		APIKey:      testAPIKey,
		APIKeyOrgID: "org-test",
	}}
	orgID, err := GetAPIKeyOrgID(s)
	assert.NoError(t, err)
	assert.Equal(t, "org-test", orgID)
}

func TestGetAPIKeyOrgID_MissingPersistedOrgReturnsError(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	s := &sideEffectingTokenStore{tokens: &entity.AuthTokens{APIKey: testAPIKey}}
	_, err := GetAPIKeyOrgID(s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auth malformed")
}

type cliAuthStore struct {
	tokens           *entity.AuthTokens
	user             *entity.User
	currentUserErr   error
	currentUserCalls int
}

func (s *cliAuthStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return s.tokens, nil
}

func (s *cliAuthStore) GetCurrentUser() (*entity.User, error) {
	s.currentUserCalls++
	if s.currentUserErr != nil {
		return nil, s.currentUserErr
	}
	return s.user, nil
}

func TestResolveCLIAuth_APIKeySkipsCurrentUser(t *testing.T) {
	s := &cliAuthStore{
		tokens: &entity.AuthTokens{APIKey: testAPIKey},
		user:   &entity.User{ID: "user-test"},
	}

	cliAuth, err := ResolveCLIAuth(s)

	assert.NoError(t, err)
	assert.True(t, cliAuth.IsAPIKey())
	assert.Nil(t, cliAuth.User())
	assert.Equal(t, 0, s.currentUserCalls)
}

func TestResolveCLIAuth_LegacyCredentialsFetchCurrentUser(t *testing.T) {
	user := &entity.User{ID: "user-test"}
	s := &cliAuthStore{
		tokens: &entity.AuthTokens{AccessToken: validToken},
		user:   user,
	}

	cliAuth, err := ResolveCLIAuth(s)

	assert.NoError(t, err)
	assert.False(t, cliAuth.IsAPIKey())
	assert.Equal(t, user, cliAuth.User())
	assert.Equal(t, 1, s.currentUserCalls)
}

func TestResolveCLIAuth_CurrentUserErrorReturnsError(t *testing.T) {
	s := &cliAuthStore{
		tokens:         &entity.AuthTokens{AccessToken: validToken},
		currentUserErr: breverrors.NewValidationError("current user failed"),
	}

	cliAuth, err := ResolveCLIAuth(s)

	assert.Error(t, err)
	assert.False(t, cliAuth.IsAPIKey())
	assert.Nil(t, cliAuth.User())
	assert.Equal(t, 1, s.currentUserCalls)
}

func TestGetFreshAccessTokenOrNil_APIKeySkipsJWTValidationAndRefresh(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{
		AccessToken:  "expired-jwt",
		APIKey:       testAPIKey,
		RefreshToken: "should-not-refresh",
	}}
	a := Auth{
		authStore: &s,
		oauth:     &MockOauth{},
		accessTokenValidator: func(_ string) (bool, error) {
			t.Fatal("api keys must not be parsed as JWTs")
			return false, nil
		},
		shouldLogin: func() (bool, error) {
			t.Fatal("api keys must not trigger login")
			return false, nil
		},
	}

	res, err := a.GetFreshAccessTokenOrNil()
	assert.NoError(t, err)
	assert.Equal(t, testAPIKey, res)
	assert.False(t, s.didSave)
}

func TestGetFreshAccessTokenOrNil_APIKeyOnlyCredentialReturnsAPIKey(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{
		APIKey: testAPIKey,
	}}
	a := Auth{
		authStore: &s,
		oauth:     &MockOauth{},
		accessTokenValidator: func(_ string) (bool, error) {
			t.Fatal("api keys must not be parsed as JWTs")
			return false, nil
		},
		shouldLogin: func() (bool, error) {
			t.Fatal("api keys must not trigger login")
			return false, nil
		},
	}

	res, err := a.GetFreshAccessTokenOrNil()
	assert.NoError(t, err)
	assert.Equal(t, testAPIKey, res)
	assert.False(t, s.didSave)
}

func TestGetFreshAccessTokenOrNil_EnvVarTakesPrecedenceOverSaved(t *testing.T) {
	t.Setenv(APIKeyEnvVar, BrevAPIKeyPrefix+"env-key")
	s := MockAuthStore{authTokens: &entity.AuthTokens{APIKey: testAPIKey}}
	a := Auth{authStore: &s, oauth: &MockOauth{}, accessTokenValidator: func(string) (bool, error) {
		t.Fatal("env key must short-circuit before touching saved credentials")
		return false, nil
	}}

	res, err := a.GetFreshAccessTokenOrNil()
	assert.NoError(t, err)
	assert.Equal(t, BrevAPIKeyPrefix+"env-key", res, "BREV_API_KEY must win over saved tokens")
}

// With no saved credential, BREV_API_KEY authenticates headless/CI commands.
func TestGetFreshAccessTokenOrNil_EnvVarFallbackWhenNoSavedTokens(t *testing.T) {
	t.Setenv(APIKeyEnvVar, testAPIKey)
	s := MockAuthStore{} // no saved tokens
	a := Auth{authStore: &s, oauth: &MockOauth{}}

	res, err := a.GetFreshAccessTokenOrNil()
	assert.NoError(t, err)
	assert.Equal(t, testAPIKey, res, "env var should be used when no credential is saved")
}

func TestGetFreshAccessTokenOrNil_EnvVarEmptyFallsThroughToSaved(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	s := MockAuthStore{authTokens: &entity.AuthTokens{APIKey: testAPIKey}}
	a := Auth{authStore: &s, oauth: &MockOauth{}}

	res, err := a.GetFreshAccessTokenOrNil()
	assert.NoError(t, err)
	assert.Equal(t, testAPIKey, res, "empty env var should fall through to saved credentials")
}

func TestLoginWithAPIKey_SavesTypedCredential(t *testing.T) {
	s := MockAuthStore{}
	a := Auth{
		authStore: &s,
		oauth:     &MockOauth{},
	}

	err := a.LoginWithAPIKey(testAPIKey, "org-test")
	assert.NoError(t, err)
	assert.True(t, s.didSave)
	assert.Equal(t, entity.AuthTokens{
		APIKey:      testAPIKey,
		APIKeyOrgID: "org-test",
	}, s.saved)
}

func TestLoginWithAPIKey_PreservesExistingJWT(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{
		AccessToken:  "existing-jwt",
		RefreshToken: "existing-refresh",
	}}
	a := Auth{
		authStore: &s,
		oauth:     &MockOauth{},
	}

	err := a.LoginWithAPIKey(testAPIKey, "org-test")
	assert.NoError(t, err)
	assert.Equal(t, entity.AuthTokens{
		AccessToken:  "existing-jwt",
		RefreshToken: "existing-refresh",
		APIKey:       testAPIKey,
		APIKeyOrgID:  "org-test",
	}, s.saved)
}

func TestLoginWithAPIKey_EmptyKeyReturnsError(t *testing.T) {
	s := MockAuthStore{}
	a := Auth{
		authStore: &s,
		oauth:     &MockOauth{},
	}

	err := a.LoginWithAPIKey("", "org-test")
	assert.Error(t, err)
	assert.False(t, s.didSave)
}

func TestStandardLogin_APIKeyCredentialDoesNotProbeOAuthProviders(t *testing.T) {
	oldStdout := os.Stdout
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})
	readPipe, writePipe, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writePipe

	_ = StandardLogin("", "", &entity.AuthTokens{
		AccessToken: "existing-jwt",
		APIKey:      testAPIKey,
	})

	assert.NoError(t, writePipe.Close())
	os.Stdout = oldStdout
	out, err := io.ReadAll(readPipe)
	assert.NoError(t, err)
	assert.Empty(t, string(out))
}

func TestSuccessNoRefreshGetFreshAccessTokenOrLogin(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{
		AccessToken:  validToken,
		RefreshToken: "rt",
	}}
	a := Auth{
		authStore: &s,
		oauth:     &MockOauth{},
		accessTokenValidator: func(s string) (bool, error) {
			return true, nil
		},
		shouldLogin: func() (bool, error) {
			return true, nil
		},
	}
	res, err := a.GetFreshAccessTokenOrLogin()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, validToken, res) {
		return
	}
	if !assert.False(t, s.didSave) {
		return
	}
}

func TestSuccessRefreshGetFreshAccessTokenOrLogin(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{
		AccessToken:  "bad",
		RefreshToken: "ref",
	}}
	a := Auth{
		authStore: &s,
		oauth: &MockOauth{
			authTokens: &entity.AuthTokens{
				AccessToken:  validToken,
				RefreshToken: "",
			},
			loginTokens: &LoginTokens{},
		},
		accessTokenValidator: func(s string) (bool, error) {
			return false, nil
		},
		shouldLogin: func() (bool, error) {
			return true, nil
		},
	}
	res, err := a.GetFreshAccessTokenOrLogin()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, validToken, res) {
		return
	}
	if !assert.True(t, s.didSave) {
		return
	}
}

func TestTokenDoesNotExistGetFreshAccessTokenOrLogin(t *testing.T) {
	o := MockOauth{
		authTokens: &entity.AuthTokens{},
		loginTokens: &LoginTokens{
			AuthTokens: entity.AuthTokens{
				AccessToken:  validToken,
				RefreshToken: "",
			},
			IDToken: "",
		},
	}
	s := MockAuthStore{
		authTokens: nil,
	}
	a := Auth{
		authStore: &s,
		oauth:     &o,
		accessTokenValidator: func(s string) (bool, error) {
			return false, nil
		},
		shouldLogin: func() (bool, error) {
			return true, nil
		},
	}
	res, err := a.GetFreshAccessTokenOrLogin()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, validToken, res) {
		return
	}
	if !assert.True(t, o.flowDone) {
		return
	}
	if !assert.True(t, s.didSave) {
		return
	}
}

func TestDenyLoginGetFreshAccessTokenOrLogin(t *testing.T) {
	s := MockAuthStore{
		authTokens: nil,
	}
	o := MockOauth{
		authTokens: &entity.AuthTokens{},
		loginTokens: &LoginTokens{
			AuthTokens: entity.AuthTokens{
				AccessToken:  "",
				RefreshToken: "",
			},
			IDToken: "",
		},
	}
	a := Auth{
		authStore: &s,
		oauth:     &o,
		accessTokenValidator: func(s string) (bool, error) {
			return false, nil
		},
		shouldLogin: func() (bool, error) {
			return false, nil
		},
	}
	res, err := a.GetFreshAccessTokenOrLogin()
	de := &breverrors.DeclineToLoginError{}
	if !assert.ErrorAs(t, err, &de) {
		return
	}
	if !assert.Empty(t, res) {
		return
	}
	if !assert.False(t, o.flowDone) {
		return
	}
	if !assert.False(t, s.didSave) {
		return
	}
	// The sentinel must remain findable through whatever wrapping the layers
	// applied — DisplayAndHandleError matches it with errors.Is.
	if !assert.True(t, errors.Is(err, de)) {
		return
	}
}

func TestFailedRefreshGetFreshAccessTokenOrLogin(t *testing.T) {
	a := Auth{
		authStore: &MockAuthStore{
			authTokens: &entity.AuthTokens{
				AccessToken:  "invalid",
				RefreshToken: "invalid",
			},
		},
		oauth: &MockOauth{
			authTokens: nil,
			loginTokens: &LoginTokens{
				AuthTokens: entity.AuthTokens{
					AccessToken:  validToken,
					RefreshToken: "",
				},
				IDToken: "",
			},
		},
		accessTokenValidator: func(s string) (bool, error) {
			return false, nil
		},
		shouldLogin: func() (bool, error) {
			return true, nil
		},
	}
	res, err := a.GetFreshAccessTokenOrLogin()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, validToken, res) {
		return
	}
}

func TestSSH(t *testing.T) {
	suite.Run(t, new(BrevAPIAuthTestSuite))
}

// countingTokenStore records how often the credential file is read.
type countingTokenStore struct {
	tokens *entity.AuthTokens
	reads  int
}

func (c *countingTokenStore) SaveAuthTokens(tokens entity.AuthTokens) error {
	c.tokens = &tokens
	return nil
}

func (c *countingTokenStore) GetAuthTokens() (*entity.AuthTokens, error) {
	c.reads++
	return c.tokens, nil
}

func (c *countingTokenStore) DeleteAuthTokens() error {
	c.tokens = nil
	return nil
}

func signedTokenExpiringAt(t *testing.T, exp time.Time) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp":   exp.Unix(),
		"email": "test@example.com",
	}).SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return token
}

// A command issues many requests; the credential must resolve once per process.
func TestGetFreshAccessTokenOrNil_ReadsSavedCredentialOncePerProcess(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	token := signedTokenExpiringAt(t, time.Now().Add(time.Hour))
	s := &countingTokenStore{tokens: &entity.AuthTokens{AccessToken: token, RefreshToken: "rt"}}
	a := NewAuth(s, &MockOauth{})

	for range 3 {
		got, err := a.GetFreshAccessTokenOrNil()
		require.NoError(t, err)
		assert.Equal(t, token, got)
	}

	assert.Equal(t, 1, s.reads)
}

// LoginAuth embeds Auth by value, so the memo must survive a value-receiver call
// through the wrapper — otherwise every request would re-read the file again.
func TestLoginAuth_SharesMemoAcrossCalls(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	token := signedTokenExpiringAt(t, time.Now().Add(time.Hour))
	s := &countingTokenStore{tokens: &entity.AuthTokens{AccessToken: token, RefreshToken: "rt"}}
	auth := NewLoginAuth(s, &MockOauth{})

	for range 3 {
		got, err := auth.GetAccessToken()
		require.NoError(t, err)
		assert.Equal(t, token, got)
	}

	assert.Equal(t, 1, s.reads)
}

// A token that expires within the skew window is never served from the memo, so
// a long-running process still refreshes instead of using a dead token.
func TestGetFreshAccessTokenOrNil_DoesNotServeTokenPastExpiry(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	s := &countingTokenStore{}
	a := NewAuth(s, &MockOauth{})
	a.accessTokenValidator = func(string) (bool, error) { return true, nil }

	s.tokens = &entity.AuthTokens{AccessToken: signedTokenExpiringAt(t, time.Now().Add(5*time.Second))}
	first, err := a.GetFreshAccessTokenOrNil()
	require.NoError(t, err)

	s.tokens = &entity.AuthTokens{AccessToken: signedTokenExpiringAt(t, time.Now().Add(time.Hour))}
	second, err := a.GetFreshAccessTokenOrNil()
	require.NoError(t, err)

	assert.NotEqual(t, first, second, "an expiring token must not be reused from the memo")
	assert.Equal(t, 2, s.reads)
}

// The memo survives until something invalidates it (login, logout, or an
// explicit invalidate after a request failed for auth reasons).
func TestInvalidateAccessTokenCache_ReResolvesCredential(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	s := &countingTokenStore{tokens: &entity.AuthTokens{APIKey: testAPIKey}}
	a := NewAuth(s, &MockOauth{})

	token, err := a.GetFreshAccessTokenOrNil()
	require.NoError(t, err)
	assert.Equal(t, testAPIKey, token)

	s.tokens = &entity.AuthTokens{APIKey: BrevAPIKeyPrefix + "rotated-key"}
	a.InvalidateAccessTokenCache()

	token, err = a.GetFreshAccessTokenOrNil()
	require.NoError(t, err)
	assert.Equal(t, BrevAPIKeyPrefix+"rotated-key", token)
}

// Stores share one credentials file (loginCmdStore / noLoginCmdStore). A
// "logged out" result must not be memoized: a login performed later in the same
// process has to be visible to credentials resolved after it.
func TestGetFreshAccessTokenOrNil_DoesNotMemoizeMissingCredential(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	store := &countingTokenStore{}
	a := NewAuth(store, &MockOauth{})

	token, err := a.GetFreshAccessTokenOrNil()
	require.NoError(t, err)
	assert.Empty(t, token)

	// The login flow writes credentials to the same store.
	require.NoError(t, store.SaveAuthTokens(entity.AuthTokens{
		AccessToken: signedTokenExpiringAt(t, time.Now().Add(time.Hour)),
	}))

	token, err = a.GetFreshAccessTokenOrNil()
	require.NoError(t, err)
	assert.NotEmpty(t, token, "credentials written after a logged-out resolution must be picked up")
}

// Logging out must not leave a usable credential behind in the memo.
func TestLogout_ClearsMemoizedCredential(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	s := &countingTokenStore{tokens: &entity.AuthTokens{APIKey: testAPIKey}}
	a := NewAuth(s, &MockOauth{})

	_, err := a.GetFreshAccessTokenOrNil()
	require.NoError(t, err)
	require.NoError(t, a.Logout())

	token, err := a.GetFreshAccessTokenOrNil()
	require.NoError(t, err)
	assert.Empty(t, token)
}
