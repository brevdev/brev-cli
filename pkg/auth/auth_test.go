package auth

import (
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

	// expiredToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImdLTXBESXlRc0ZXSF9zYWdiT2oyViJ9.eyJpc3MiOiJodHRwczovL2JyZXZkZXYudXMuYXV0aDAuY29tLyIsInN1YiI6Imdvb2dsZS1vYXV0aDJ8MTAxNzY0NjMwNTEwODYxNDk5MTgwIiwiYXVkIjpbImh0dHBzOi8vYnJldmRldi51cy5hdXRoMC5jb20vYXBpL3YyLyIsImh0dHBzOi8vYnJldmRldi51cy5hdXRoMC5jb20vdXNlcmluZm8iXSwiaWF0IjoxNjM4NTYyMzY4LCJleHAiOjE2Mzg2NDg3NjgsImF6cCI6IkphcUpSTEVzZGF0NXc3VGIwV3FtVHh6SWVxd3FlcG1rIiwic2NvcGUiOiJvcGVuaWQgcHJvZmlsZSBlbWFpbCBvZmZsaW5lX2FjY2VzcyJ9.YCiO-som26ehT91qGAX5ZfrtVg4eYwamnlMRoCuUljXmg8Nf-ArDyoG32CqZkQ6YJ5XnzrVX9bVk5ZNHP_AFSE9SJvYL6MchoN09nR84WTbevRBCtZedIZUk5ULg6rWo5mszGr-S2gi08od4iTzXtKySPx1JnT60muRj_k9VV3MyixqvngEz5NvmFDdA8glGes5_iOuiBidmjOJzi_CVfKJ9s48BhlxzciSXFC0_DUBnT9OThjYjUP-22ohOuWwJWomRUv6gMSq78hJOALc330LwvmEsLdzlP7a3otIYM43hTtAVJ9QEL6M08GKqm3PdikzTxiGdfuQUhgMDlXygbQ"
	// res, err := isAccessTokenValid(expiredToken)
	// if !assert.Nil(t, err) {
	// 	return
	// }
	// if !assert.False(t, res) {
	// 	return
	// }
}

type MockAuthStore struct {
	authTokens *entity.AuthTokens
	saved      entity.AuthTokens
	didSave    bool
}

func (m *MockAuthStore) SaveAuthTokens(tokens entity.AuthTokens) error {
	m.saved = tokens
	m.didSave = true
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
}

func (s *sideEffectingTokenStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return s.tokens, nil
}

func (s *sideEffectingTokenStore) GetAccessToken() (string, error) {
	s.getAccessTokenCalled = true
	return testAPIKey, nil
}

func TestIsAPIKeyAuthStore_ReadsSavedTokensWithoutAccessTokenSideEffects(t *testing.T) {
	s := &sideEffectingTokenStore{
		tokens: &entity.AuthTokens{APIKey: testAPIKey},
	}

	assert.True(t, IsAPIKeyAuthStore(s))
	assert.False(t, s.getAccessTokenCalled)
}

func TestIsAPIKeyAuthStore_LegacyCredentialsAreNotAPIKeyAuth(t *testing.T) {
	s := &sideEffectingTokenStore{
		tokens: &entity.AuthTokens{
			AccessToken:  validToken,
			RefreshToken: "refresh",
		},
	}

	assert.False(t, IsAPIKeyAuthStore(s))
	assert.False(t, s.getAccessTokenCalled)
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

func TestLoginWithAPIKey_EmptyOrgIDReturnsError(t *testing.T) {
	s := MockAuthStore{}
	a := Auth{
		authStore: &s,
		oauth:     &MockOauth{},
	}

	err := a.LoginWithAPIKey(testAPIKey, "")
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
