package auth

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
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
	assert.Contains(t, err.Error(), "expected API key to resolve to exactly one organization, got 0")
}

func TestResolveEnvAPIKeyOrg_MultipleOrgsReturnsError(t *testing.T) {
	t.Setenv(APIKeyEnvVar, testAPIKey)
	s := &sideEffectingTokenStore{orgs: []entity.Organization{
		{ID: "org-1", Name: "One"}, {ID: "org-2", Name: "Two"},
	}}
	_, err := ResolveEnvAPIKeyOrg(s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected API key to resolve to exactly one organization, got 2")
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
		&s,
		&MockOauth{}, func(_ string) (bool, error) {
			t.Fatal("api keys must not be parsed as JWTs")
			return false, nil
		},
		func() (bool, error) {
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
		&s,
		&MockOauth{}, func(_ string) (bool, error) {
			t.Fatal("api keys must not be parsed as JWTs")
			return false, nil
		},
		func() (bool, error) {
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
		&s,
		&MockOauth{}, func(s string) (bool, error) {
			return true, nil
		},
		func() (bool, error) {
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
		&s, &MockOauth{
			authTokens: &entity.AuthTokens{
				AccessToken:  validToken,
				RefreshToken: "",
			},
			loginTokens: &LoginTokens{},
		}, func(s string) (bool, error) {
			return false, nil
		}, func() (bool, error) {
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
		&s, &o, func(s string) (bool, error) {
			return false, nil
		}, func() (bool, error) {
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
		&s, &o, func(s string) (bool, error) {
			return false, nil
		}, func() (bool, error) {
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
		&MockAuthStore{
			authTokens: &entity.AuthTokens{
				AccessToken:  "invalid",
				RefreshToken: "invalid",
			},
		}, &MockOauth{
			authTokens: nil,
			loginTokens: &LoginTokens{
				AuthTokens: entity.AuthTokens{
					AccessToken:  validToken,
					RefreshToken: "",
				},
				IDToken: "",
			},
		}, func(s string) (bool, error) {
			return false, nil
		}, func() (bool, error) {
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
