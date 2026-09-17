package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// MockAuthStore records saves so tests can assert persistence counts.
type MockAuthStore struct {
	authTokens *entity.AuthTokens
	saved      entity.AuthTokens
	didSave    bool
	saveCalls  int
	saveErr    error
}

func (m *MockAuthStore) SaveAuthTokens(tokens entity.AuthTokens) error {
	m.saveCalls++
	if m.saveErr != nil {
		return m.saveErr
	}
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

// MockOauth counts device-flow and refresh calls and can fail either.
type MockOauth struct {
	authTokens   *entity.AuthTokens
	loginTokens  *LoginTokens
	flowErr      error
	refreshErr   error
	flowDone     bool
	flowCalls    int
	refreshCalls int
}

func (m *MockOauth) GetCredentialProvider() entity.CredentialProvider {
	return "mock"
}

func (m *MockOauth) IsTokenValid(token string) bool {
	return true
}

func (m *MockOauth) DoDeviceAuthFlow(_ func(string, string)) (*LoginTokens, error) {
	m.flowCalls++
	if m.flowErr != nil {
		return nil, m.flowErr
	}
	m.flowDone = true
	return m.loginTokens, nil
}

func (m *MockOauth) GetNewAuthTokensWithRefresh(_ string) (*entity.AuthTokens, error) {
	m.refreshCalls++
	if m.refreshErr != nil {
		return nil, m.refreshErr
	}
	return m.authTokens, nil
}

const (
	validToken = "abc"
	testAPIKey = BrevAPIKeyPrefix + "test-key"
)

// testJWT builds a parseable JWT with a valid issued-at claim; isAccessTokenValid
// only inspects claims, so no signature is needed.
func testJWT(t *testing.T) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"iat":%d}`, time.Now().Unix())))
	return header + "." + claims + ".sig"
}

func TestIsBrevAPIKey(t *testing.T) {
	assert.True(t, IsBrevAPIKey(testAPIKey))
	assert.True(t, IsBrevAPIKey("  "+testAPIKey+"  "))
	assert.False(t, IsBrevAPIKey("bakery-token"))
	assert.False(t, IsBrevAPIKey("jwt-token"))
	assert.False(t, IsBrevAPIKey(""))
}

type sideEffectingTokenStore struct {
	tokens               *entity.AuthTokens
	orgs                 []entity.Organization
	listOrganizationsErr error
}

func (s *sideEffectingTokenStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return s.tokens, nil
}

func (s *sideEffectingTokenStore) ListOrganizations() ([]entity.Organization, error) {
	return s.orgs, s.listOrganizationsErr
}

func TestIsAPIKeyAuthStore_ReadsSavedTokens(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	assert.True(t, IsAPIKeyAuthStore(&sideEffectingTokenStore{tokens: &entity.AuthTokens{APIKey: testAPIKey}}))
}

func TestIsAPIKeyAuthStore_LegacyCredentialsAreNotAPIKeyAuth(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	s := &sideEffectingTokenStore{tokens: &entity.AuthTokens{AccessToken: validToken, RefreshToken: "refresh"}}
	assert.False(t, IsAPIKeyAuthStore(s))
}

func TestIsAPIKeyAuthStore_EnvKeyIsAPIKeyEvenWhenNotPersisted(t *testing.T) {
	t.Setenv(APIKeyEnvVar, testAPIKey)
	assert.True(t, IsAPIKeyAuthStore(&sideEffectingTokenStore{tokens: nil}))
}

func TestResolveEnvAPIKeyOrg(t *testing.T) {
	oneOrg := []entity.Organization{{ID: "org-realtime", Name: "Realtime Org"}}
	twoOrgs := []entity.Organization{{ID: "org-1", Name: "One"}, {ID: "org-2", Name: "Two"}}

	tests := []struct {
		name    string
		env     string
		store   *sideEffectingTokenStore
		wantOrg *entity.Organization
		wantErr string
	}{
		{name: "resolves the key's org in real time", env: testAPIKey, store: &sideEffectingTokenStore{orgs: oneOrg}, wantOrg: &oneOrg[0]},
		{name: "no orgs is invalid", env: testAPIKey, store: &sideEffectingTokenStore{}, wantErr: "api key invalid"},
		{name: "multiple orgs is invalid", env: testAPIKey, store: &sideEffectingTokenStore{orgs: twoOrgs}, wantErr: "api key invalid"},
		{name: "list error propagates", env: testAPIKey, store: &sideEffectingTokenStore{listOrganizationsErr: errors.New("boom")}, wantErr: "boom"},
		{name: "no env key resolves nothing", env: "", store: &sideEffectingTokenStore{orgs: oneOrg}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(APIKeyEnvVar, tt.env)
			org, err := ResolveEnvAPIKeyOrg(tt.store)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOrg, org)
		})
	}
}

func TestGetAPIKeyOrgID(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")

	orgID, err := GetAPIKeyOrgID(&sideEffectingTokenStore{tokens: &entity.AuthTokens{APIKey: testAPIKey, APIKeyOrgID: "org-test"}})
	require.NoError(t, err)
	assert.Equal(t, "org-test", orgID)

	_, err = GetAPIKeyOrgID(&sideEffectingTokenStore{tokens: &entity.AuthTokens{APIKey: testAPIKey}})
	require.Error(t, err)
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

func TestResolveCLIAuth(t *testing.T) {
	user := &entity.User{ID: "user-test"}

	s := &cliAuthStore{tokens: &entity.AuthTokens{APIKey: testAPIKey}, user: user}
	cliAuth, err := ResolveCLIAuth(s)
	require.NoError(t, err)
	assert.True(t, cliAuth.IsAPIKey())
	assert.Nil(t, cliAuth.User())
	assert.Zero(t, s.currentUserCalls, "API-key auth must skip the current-user lookup")

	s = &cliAuthStore{tokens: &entity.AuthTokens{AccessToken: validToken}, user: user}
	cliAuth, err = ResolveCLIAuth(s)
	require.NoError(t, err)
	assert.False(t, cliAuth.IsAPIKey())
	assert.Equal(t, user, cliAuth.User())
	assert.Equal(t, 1, s.currentUserCalls)

	s = &cliAuthStore{tokens: &entity.AuthTokens{AccessToken: validToken}, currentUserErr: breverrors.NewValidationError("current user failed")}
	_, err = ResolveCLIAuth(s)
	require.Error(t, err)
	assert.Equal(t, 1, s.currentUserCalls)
}

// resolutionCases drives TestActiveCredentialResolution. touchJWT false
// asserts the JWT is never validated (the API-key path wins first).
var resolutionCases = []struct {
	name     string
	env      string
	tokens   *entity.AuthTokens
	touchJWT bool
	jwtValid bool
	want     string
	wantKind CredentialKind
}{
	{name: "env key wins over saved credentials", env: BrevAPIKeyPrefix + "env-key", tokens: &entity.AuthTokens{APIKey: testAPIKey}, want: BrevAPIKeyPrefix + "env-key", wantKind: CredentialAPIKey},
	{name: "env key fallback when nothing saved", env: testAPIKey, want: testAPIKey, wantKind: CredentialAPIKey},
	{name: "saved API key only", tokens: &entity.AuthTokens{APIKey: testAPIKey}, want: testAPIKey, wantKind: CredentialAPIKey},
	{name: "saved API key skips JWT validation and refresh", tokens: &entity.AuthTokens{AccessToken: "expired-jwt", RefreshToken: "should-not-refresh", APIKey: testAPIKey}, want: testAPIKey, wantKind: CredentialAPIKey},
	{name: "legacy mixed file keeps API key active", tokens: jwtPlusKey, want: testAPIKey, wantKind: CredentialAPIKey},
	{name: "preferred api_key keeps the key active", tokens: withPref(jwtPlusKey, CredentialAPIKeyPreference), want: testAPIKey, wantKind: CredentialAPIKey},
	{name: "preferred user uses the JWT over the saved API key", tokens: withPref(jwtPlusKey, CredentialUserPreference), touchJWT: true, jwtValid: true, want: validToken, wantKind: CredentialUserJWT},
	{name: "JWT only", tokens: jwtOnly, touchJWT: true, jwtValid: true, want: validToken, wantKind: CredentialUserJWT},
	{name: "preferred user with unusable JWT returns nothing, not the API key", tokens: withPref(&entity.AuthTokens{AccessToken: "expired", APIKey: testAPIKey, APIKeyOrgID: "org-test"}, CredentialUserPreference), touchJWT: true, want: "", wantKind: CredentialUserJWT},
}

var (
	jwtOnly    = &entity.AuthTokens{AccessToken: validToken, RefreshToken: "rt"}
	jwtPlusKey = &entity.AuthTokens{AccessToken: validToken, RefreshToken: "rt", APIKey: testAPIKey, APIKeyOrgID: "org-test"}
)

func withPref(tokens *entity.AuthTokens, pref string) *entity.AuthTokens {
	c := *tokens
	c.PreferredCredential = pref
	return &c
}

func TestActiveCredentialResolution(t *testing.T) {
	for _, tt := range resolutionCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(APIKeyEnvVar, tt.env)
			s := MockAuthStore{authTokens: tt.tokens}
			o := MockOauth{}
			validator := func(string) (bool, error) { return tt.jwtValid, nil }
			if !tt.touchJWT {
				validator = func(string) (bool, error) {
					t.Fatal("JWT must not be validated on the API-key path")
					return false, nil
				}
			}
			na := NewNoLoginAuth(&s, &o)
			na.WithAccessTokenValidator(validator)

			cred, err := na.GetCredential()
			require.NoError(t, err)
			assert.Equal(t, tt.want, cred.Token)
			assert.Equal(t, tt.wantKind, cred.Kind)
			assert.Zero(t, o.refreshCalls, "resolution must not refresh")
			assert.False(t, s.didSave)
		})
	}
}

func TestLoginAuth_RequireUserPolicy(t *testing.T) {
	loginTokens := &LoginTokens{AuthTokens: entity.AuthTokens{AccessToken: validToken, RefreshToken: "rt"}}

	tests := []struct {
		name         string
		env          string
		tokens       *entity.AuthTokens
		jwtValid     bool
		shouldLogin  bool
		want         string
		wantFlowCall int
		wantDecline  bool
	}{
		{name: "valid JWT used over saved API key", tokens: jwtPlusKey, jwtValid: true, want: validToken},
		{name: "valid JWT reused even when env API key is set", env: testAPIKey, tokens: jwtOnly, jwtValid: true, want: validToken},
		{name: "API key only prompts login", tokens: &entity.AuthTokens{APIKey: testAPIKey, APIKeyOrgID: "org-test"}, shouldLogin: true, want: validToken, wantFlowCall: 1},
		{name: "env API key ignored, prompts login", env: testAPIKey, shouldLogin: true, want: validToken, wantFlowCall: 1},
		{name: "no credential prompts login", shouldLogin: true, want: validToken, wantFlowCall: 1},
		{name: "declined login surfaces DeclineToLoginError", wantDecline: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(APIKeyEnvVar, tt.env)
			s := MockAuthStore{authTokens: tt.tokens}
			o := MockOauth{loginTokens: loginTokens}
			la := NewUserLoginAuth(&s, &o)
			la.WithAccessTokenValidator(func(string) (bool, error) { return tt.jwtValid, nil })
			la.WithShouldLogin(func() (bool, error) { return tt.shouldLogin, nil })

			cred, err := la.GetCredential()
			if tt.wantDecline {
				var decline *breverrors.DeclineToLoginError
				require.ErrorAs(t, err, &decline)
				assert.Zero(t, o.flowCalls, "a declined prompt must not run the device flow")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, cred.Token)
			assert.Equal(t, CredentialUserJWT, cred.Kind, "RequireUser only yields user credentials")
			assert.Equal(t, tt.wantFlowCall, o.flowCalls)
		})
	}
}

// When the preferred user JWT is unusable, the active-credential login auth
// prompts for a fresh login rather than silently using the saved API key.
func TestGetFreshAccessTokenOrLogin_PreferredUserUnusableJWTPromptsLogin(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	o := MockOauth{loginTokens: &LoginTokens{
		AuthTokens: entity.AuthTokens{AccessToken: validToken, RefreshToken: "rt"},
	}}
	s := MockAuthStore{authTokens: &entity.AuthTokens{
		AccessToken:         "expired",
		APIKey:              testAPIKey,
		APIKeyOrgID:         "org-test",
		PreferredCredential: CredentialUserPreference,
	}}
	a := Auth{
		authStore:            &s,
		oauth:                &o,
		accessTokenValidator: func(string) (bool, error) { return false, nil },
		shouldLogin:          func() (bool, error) { return true, nil },
	}

	res, err := a.GetFreshAccessTokenOrLogin()
	assert.NoError(t, err)
	assert.Equal(t, validToken, res)
	assert.True(t, o.flowDone, "unusable preferred JWT must prompt for a fresh login")
}

// ActivateUserCredential flips the active credential while preserving the
// other one, so a later login can reactivate it.
func TestActivateUserCredential_PreservesOtherCredential(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{
		AccessToken:  validToken,
		RefreshToken: "rt",
		APIKey:       testAPIKey,
		APIKeyOrgID:  "org-test",
	}}
	a := Auth{authStore: &s, oauth: &MockOauth{}}

	err := a.ActivateUserCredential()
	assert.NoError(t, err)
	assert.Equal(t, entity.AuthTokens{
		AccessToken:         validToken,
		RefreshToken:        "rt",
		APIKey:              testAPIKey,
		APIKeyOrgID:         "org-test",
		PreferredCredential: CredentialUserPreference,
	}, s.saved)
}

// IsAPIKeyAuthStore reflects the preferred credential, not mere presence.
func TestIsAPIKeyAuthStore_PrefersExplicitPreference(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "")
	withAPIKey := func(pref string) *sideEffectingTokenStore {
		return &sideEffectingTokenStore{tokens: &entity.AuthTokens{
			AccessToken:         validToken,
			RefreshToken:        "rt",
			APIKey:              testAPIKey,
			APIKeyOrgID:         "org-test",
			PreferredCredential: pref,
		}}
	}
	assert.False(t, IsAPIKeyAuthStore(withAPIKey(CredentialUserPreference)), "preferred user is not API-key auth")
	assert.True(t, IsAPIKeyAuthStore(withAPIKey(CredentialAPIKeyPreference)), "preferred api_key is API-key auth")
	assert.True(t, IsAPIKeyAuthStore(withAPIKey("")), "legacy mixed file keeps API key as active")
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
		APIKey:              testAPIKey,
		APIKeyOrgID:         "org-test",
		PreferredCredential: CredentialAPIKeyPreference,
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
		AccessToken:         "existing-jwt",
		RefreshToken:        "existing-refresh",
		APIKey:              testAPIKey,
		APIKeyOrgID:         "org-test",
		PreferredCredential: CredentialAPIKeyPreference,
	}, s.saved)
}

// Login (device flow) must merge the fresh JWT into the saved record, keeping
// any API key and the current preference intact.
func TestLogin_MergesAndPreservesAPIKeyAndPreference(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{
		APIKey:              testAPIKey,
		APIKeyOrgID:         "org-test",
		PreferredCredential: CredentialAPIKeyPreference,
	}}
	o := MockOauth{loginTokens: &LoginTokens{
		AuthTokens: entity.AuthTokens{AccessToken: validToken, RefreshToken: "rt"},
	}}
	a := Auth{authStore: &s, oauth: &o}

	_, err := a.Login(false)
	assert.NoError(t, err)
	assert.Equal(t, entity.AuthTokens{
		AccessToken:         validToken,
		RefreshToken:        "rt",
		APIKey:              testAPIKey,
		APIKeyOrgID:         "org-test",
		PreferredCredential: CredentialAPIKeyPreference,
	}, s.saved, "login must preserve the API key and the current preference")
}

// Refreshing an expired JWT must merge the new tokens into the saved record,
// preserving the API key and preference. The refresh token rotates only when
// the provider returns one; an omitted refresh token must keep the previous one.
func TestRefresh_MergesAndPreservesAPIKey(t *testing.T) {
	const oldRefreshToken = "rt"

	tests := []struct {
		name string
		// refreshToken is what the provider returns; empty means it omitted one.
		refreshToken string
		wantRefresh  string
	}{
		{
			name:         "provider returns a new refresh token, replacing the old one",
			refreshToken: "rt-new",
			wantRefresh:  "rt-new",
		},
		{
			name:         "provider omits a refresh token, retaining the old one",
			refreshToken: "",
			wantRefresh:  oldRefreshToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := MockAuthStore{authTokens: &entity.AuthTokens{
				AccessToken:         "bad",
				RefreshToken:        oldRefreshToken,
				APIKey:              testAPIKey,
				APIKeyOrgID:         "org-test",
				PreferredCredential: CredentialUserPreference,
			}}
			o := MockOauth{authTokens: &entity.AuthTokens{
				AccessToken:  validToken,
				RefreshToken: tt.refreshToken,
			}}
			a := Auth{
				authStore:            &s,
				oauth:                &o,
				accessTokenValidator: func(string) (bool, error) { return false, nil },
			}

			token, err := a.GetFreshAccessTokenOrNil()
			require.NoError(t, err)
			assert.Equal(t, validToken, token)
			assert.Equal(t, 1, o.refreshCalls)
			assert.Equal(t, entity.AuthTokens{
				AccessToken:         validToken,
				RefreshToken:        tt.wantRefresh,
				APIKey:              testAPIKey,
				APIKeyOrgID:         "org-test",
				PreferredCredential: CredentialUserPreference,
			}, s.saved, "refresh must rotate the refresh token as returned and preserve the API key")
		})
	}
}

// LoginWithToken merges the token into the saved record while preserving an API
// key. It does not activate the preference itself — the explicit login flow
// does that once, separately.
func TestLoginWithToken_MergesAndPreservesAPIKey(t *testing.T) {
	jwt := testJWT(t)
	newExisting := func() *entity.AuthTokens {
		return &entity.AuthTokens{
			APIKey:              testAPIKey,
			APIKeyOrgID:         "org-test",
			PreferredCredential: CredentialAPIKeyPreference,
		}
	}

	tests := []struct {
		name  string
		token string
		want  entity.AuthTokens
	}{
		{
			name:  "parseable JWT stored as the access token",
			token: jwt,
			want: entity.AuthTokens{
				AccessToken:         jwt,
				RefreshToken:        "auto-login",
				APIKey:              testAPIKey,
				APIKeyOrgID:         "org-test",
				PreferredCredential: CredentialAPIKeyPreference,
			},
		},
		{
			name:  "opaque token stored as the refresh token",
			token: "opaque-token",
			want: entity.AuthTokens{
				AccessToken:         "auto-login",
				RefreshToken:        "opaque-token",
				APIKey:              testAPIKey,
				APIKeyOrgID:         "org-test",
				PreferredCredential: CredentialAPIKeyPreference,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := MockAuthStore{authTokens: newExisting()}
			a := Auth{authStore: &s, oauth: &MockOauth{}}

			require.NoError(t, a.LoginWithToken(tt.token))
			assert.Equal(t, tt.want, s.saved)
			assert.Equal(t, 1, s.saveCalls, "a token login must write the merged record exactly once")
		})
	}
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

// TestLoginAuth_PreferActivePolicy covers the active-credential login flow:
// reuse a valid JWT, refresh an expired one, prompt when none exists, and
// surface the decline sentinel findable via errors.Is.
func TestLoginAuth_PreferActivePolicy(t *testing.T) {
	loginTokens := &LoginTokens{AuthTokens: entity.AuthTokens{AccessToken: validToken, RefreshToken: "rt"}}

	tests := []struct {
		name          string
		tokens        *entity.AuthTokens
		refreshTokens *entity.AuthTokens
		jwtValid      bool
		shouldLogin   bool
		want          string
		wantFlowCall  int
		wantRefresh   int
		wantDecline   bool
	}{
		{
			name:     "valid JWT needs no refresh or login",
			tokens:   &entity.AuthTokens{AccessToken: validToken, RefreshToken: "rt"},
			jwtValid: true,
			want:     validToken,
		},
		{
			name:          "expired JWT refreshes",
			tokens:        &entity.AuthTokens{AccessToken: "expired", RefreshToken: "rt"},
			refreshTokens: &entity.AuthTokens{AccessToken: validToken},
			want:          validToken,
			wantRefresh:   1,
		},
		{
			name:         "no credential prompts login",
			want:         validToken,
			shouldLogin:  true,
			wantFlowCall: 1,
		},
		{
			name:        "declined login surfaces the sentinel",
			shouldLogin: false,
			wantDecline: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(APIKeyEnvVar, "")
			s := MockAuthStore{authTokens: tt.tokens}
			o := MockOauth{authTokens: tt.refreshTokens, loginTokens: loginTokens}
			la := NewLoginAuth(&s, &o)
			la.WithAccessTokenValidator(func(string) (bool, error) { return tt.jwtValid, nil })
			la.WithShouldLogin(func() (bool, error) { return tt.shouldLogin, nil })

			token, err := la.GetAccessToken()
			if tt.wantDecline {
				var decline *breverrors.DeclineToLoginError
				require.ErrorAs(t, err, &decline)
				assert.True(t, errors.Is(err, decline), "sentinel must survive wrapping")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, token)
			assert.Equal(t, tt.wantFlowCall, o.flowCalls)
			assert.Equal(t, tt.wantRefresh, o.refreshCalls)
		})
	}
}

// A refresh that returns no tokens falls through to an interactive login.
func TestRefreshReturnsNoTokensThenLogsIn(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{AccessToken: "expired", RefreshToken: "rt"}}
	o := MockOauth{loginTokens: &LoginTokens{
		AuthTokens: entity.AuthTokens{AccessToken: validToken, RefreshToken: "rt"},
	}}
	a := Auth{
		authStore:            &s,
		oauth:                &o,
		accessTokenValidator: func(string) (bool, error) { return false, nil },
		shouldLogin:          func() (bool, error) { return true, nil },
	}

	res, err := a.GetFreshAccessTokenOrLogin()
	require.NoError(t, err)
	assert.Equal(t, validToken, res)
	assert.Equal(t, 1, o.refreshCalls)
	assert.Equal(t, 1, o.flowCalls)
}

// A refresh error must propagate, not silently fall through to login.
func TestRefreshErrorPropagates(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{AccessToken: "expired", RefreshToken: "rt"}}
	o := MockOauth{refreshErr: errors.New("refresh boom")}
	a := Auth{
		authStore:            &s,
		oauth:                &o,
		accessTokenValidator: func(string) (bool, error) { return false, nil },
		shouldLogin:          func() (bool, error) { return true, nil },
	}

	_, err := a.GetFreshAccessTokenOrLogin()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "refresh boom")
	assert.Equal(t, 1, o.refreshCalls)
	assert.Zero(t, o.flowCalls, "a refresh error must not fall through to login")
}
