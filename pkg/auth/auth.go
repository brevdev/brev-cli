package auth

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/fatih/color"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/browser"
)

type LoginAuth struct {
	Auth
	policy credentialPolicy
}

// NewLoginAuth returns a login auth that resolves the active credential
// (policyPreferActive) and prompts for login when none is available.
func NewLoginAuth(authStore AuthStore, oauth OAuth) *LoginAuth {
	return &LoginAuth{
		Auth:   *NewAuth(authStore, oauth),
		policy: policyPreferActive,
	}
}

// NewUserLoginAuth returns a login auth that resolves only the user credential
// (policyRequireUser) and prompts for login when none is available. Operations
// that need a full user (e.g. switching orgs) use this: an API key, scoped to
// a single org, can never satisfy them.
func NewUserLoginAuth(authStore AuthStore, oauth OAuth) *LoginAuth {
	return &LoginAuth{
		Auth:   *NewAuth(authStore, oauth),
		policy: policyRequireUser,
	}
}

func (l LoginAuth) GetCredential() (Credential, error) {
	cred, err := l.resolveCredential(l.policy)
	if err != nil {
		return Credential{}, breverrors.WrapAndTrace(err)
	}
	if cred.Token == "" {
		lt, err := l.PromptForLogin()
		if err != nil {
			return Credential{}, breverrors.WrapAndTrace(err)
		}
		cred = Credential{Token: lt.AccessToken, Kind: CredentialUserJWT}
	}
	return cred, nil
}

func (l LoginAuth) GetAccessToken() (string, error) {
	cred, err := l.GetCredential()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return cred.Token, nil
}

type NoLoginAuth struct {
	Auth
}

func NewNoLoginAuth(authStore AuthStore, oauth OAuth) *NoLoginAuth {
	return &NoLoginAuth{
		Auth: *NewAuth(authStore, oauth),
	}
}

func (l NoLoginAuth) GetCredential() (Credential, error) {
	cred, err := l.resolveCredential(policyPreferActive)
	if err != nil {
		return Credential{}, breverrors.WrapAndTrace(err)
	}
	return cred, nil
}

func (l NoLoginAuth) GetAccessToken() (string, error) {
	cred, err := l.GetCredential()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return cred.Token, nil
}

type AuthStore interface {
	SaveAuthTokens(tokens entity.AuthTokens) error
	GetAuthTokens() (*entity.AuthTokens, error)
	DeleteAuthTokens() error
}

type OAuth interface {
	DoDeviceAuthFlow(onStateRetrieved func(url string, code string)) (*LoginTokens, error)
	GetNewAuthTokensWithRefresh(refreshToken string) (*entity.AuthTokens, error)
	GetCredentialProvider() entity.CredentialProvider
	IsTokenValid(token string) bool
}

type OAuthRetriever struct {
	oauths []OAuth
}

func NewOAuthRetriever(oauths []OAuth) *OAuthRetriever {
	return &OAuthRetriever{
		oauths: oauths,
	}
}

func (o *OAuthRetriever) GetByProvider(provider entity.CredentialProvider) (OAuth, error) {
	for _, oauth := range o.oauths {
		if oauth.GetCredentialProvider() == provider {
			return oauth, nil
		}
	}
	return nil, fmt.Errorf("no oauth found for provider %s", provider)
}

func (o *OAuthRetriever) GetByToken(token string) (OAuth, error) {
	for _, oauth := range o.oauths {
		if oauth.IsTokenValid(token) {
			return oauth, nil
		}
	}
	return nil, fmt.Errorf("no oauth found for token")
}

type Auth struct {
	authStore            AuthStore
	oauth                OAuth
	accessTokenValidator func(string) (bool, error)
	shouldLogin          func() (bool, error)
}

const BrevAPIKeyPrefix = "bak-"

const APIKeyEnvVar = "BREV_API_KEY"

// CredentialKind identifies what a credential is, so callers can branch on the
// credential actually in use rather than inferring from the token store.
type CredentialKind int

const (
	CredentialUserJWT CredentialKind = iota
	CredentialAPIKey
)

type Credential struct {
	Token string
	Kind  CredentialKind
}

// credentialPolicy selects how a token is resolved.
type credentialPolicy int

const (
	// policyPreferActive uses whichever credential the user has activated
	policyPreferActive credentialPolicy = iota
	// policyRequireUser resolves only a user JWT, ignoring API keys entirely
	// useful for cross org commands like "set"
	policyRequireUser
)

// PreferredCredential values stored in entity.AuthTokens.PreferredCredential.
const (
	CredentialUserPreference   = "user"
	CredentialAPIKeyPreference = "api_key"
)

const MissingAPIKeyOrgIDMessage = "auth malformed; run brev login --api-key <api-key>"

type APIKeyAuthStore interface {
	GetAuthTokens() (*entity.AuthTokens, error)
}

// OrgLister lists the organizations available to the current credential.
type OrgLister interface {
	ListOrganizations() ([]entity.Organization, error)
}

type CurrentUserAuthStore interface {
	APIKeyAuthStore
	GetCurrentUser() (*entity.User, error)
}

type CLIAuth struct {
	apiKey bool
	user   *entity.User
}

func (a CLIAuth) IsAPIKey() bool {
	return a.apiKey
}

func (a CLIAuth) User() *entity.User {
	return a.user
}

func ResolveCLIAuth(store CurrentUserAuthStore) (CLIAuth, error) {
	if IsAPIKeyAuthStore(store) {
		return CLIAuth{apiKey: true}, nil
	}
	user, err := store.GetCurrentUser()
	if err != nil {
		return CLIAuth{}, breverrors.WrapAndTrace(err)
	}
	return CLIAuth{user: user}, nil
}

func IsBrevAPIKey(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), BrevAPIKeyPrefix)
}

func IsAPIKeyAuthStore(authTokensProvider APIKeyAuthStore) bool {
	if strings.TrimSpace(os.Getenv(APIKeyEnvVar)) != "" {
		return true
	}
	tokens, err := authTokensProvider.GetAuthTokens()
	if err != nil || tokens == nil {
		return false
	}
	if strings.TrimSpace(tokens.PreferredCredential) == CredentialUserPreference {
		return false
	}
	return IsBrevAPIKey(tokens.APIKey)
}

func ResolveEnvAPIKeyOrg(orgLister OrgLister) (*entity.Organization, error) {
	if strings.TrimSpace(os.Getenv(APIKeyEnvVar)) == "" {
		return nil, nil
	}
	return ResolveAPIKeyOrganization(orgLister)
}

func ResolveAPIKeyOrganization(orgLister OrgLister) (*entity.Organization, error) {
	orgs, err := orgLister.ListOrganizations()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if len(orgs) != 1 {
		return nil, breverrors.New("api key invalid")
	}
	return &orgs[0], nil
}

func GetAPIKeyOrgID(authTokensProvider APIKeyAuthStore) (string, error) {
	tokens, err := authTokensProvider.GetAuthTokens()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if tokens == nil {
		return "", breverrors.NewValidationError(MissingAPIKeyOrgIDMessage)
	}
	orgID := strings.TrimSpace(tokens.APIKeyOrgID)
	if orgID == "" {
		return "", breverrors.NewValidationError(MissingAPIKeyOrgIDMessage)
	}
	return orgID, nil
}

func NewAuth(authStore AuthStore, oauth OAuth) *Auth {
	return &Auth{
		authStore:            authStore,
		oauth:                oauth,
		accessTokenValidator: isAccessTokenValid,
		shouldLogin:          shouldLogin,
	}
}

func (t *Auth) WithAccessTokenValidator(val func(string) (bool, error)) *Auth {
	t.accessTokenValidator = val
	return t
}

func (t *Auth) WithShouldLogin(fn func() (bool, error)) *Auth {
	t.shouldLogin = fn
	return t
}

// Gets fresh access token and prompts for login and saves to store
func (t Auth) GetFreshAccessTokenOrLogin() (string, error) {
	token, err := t.GetFreshAccessTokenOrNil()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if token == "" {
		lt, err := t.PromptForLogin()
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		token = lt.AccessToken
	}
	return token, nil
}

// Gets fresh access token or returns nil and saves to store
func (t Auth) GetFreshAccessTokenOrNil() (string, error) {
	cred, err := t.resolveCredential(policyPreferActive)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return cred.Token, nil
}

// resolveCredential selects the credential to use for the given policy.
// It never prompts for login; callers decide whether to prompt when it returns
// an empty credential.
//
//	policyPreferActive: BREV_API_KEY env var, else the preferred_credential
//	saved in the token store. Legacy files (no preference field) keep the old
//	rule: an API key wins over a JWT.
//	policyRequireUser: a user JWT only, skipping API keys entirely.
func (t Auth) resolveCredential(policy credentialPolicy) (Credential, error) {
	if policy != policyRequireUser {
		if key := strings.TrimSpace(os.Getenv(APIKeyEnvVar)); key != "" {
			return Credential{Token: key, Kind: CredentialAPIKey}, nil
		}
	}

	tokens, err := t.getSavedTokensOrNil()
	if err != nil {
		return Credential{}, breverrors.WrapAndTrace(err)
	}
	if tokens == nil {
		return Credential{}, nil
	}

	apiKey := strings.TrimSpace(tokens.APIKey)

	// policyRequireUser: only a user JWT counts.
	if policy == policyRequireUser {
		if tokens.AccessToken == "" && tokens.RefreshToken == "" {
			return Credential{}, nil
		}
		userToken, err := t.resolveUserAccessToken(tokens)
		if err != nil {
			return Credential{}, breverrors.WrapAndTrace(err)
		}
		if userToken == "" {
			return Credential{}, nil // no usable user JWT: caller prompts
		}
		return Credential{Token: userToken, Kind: CredentialUserJWT}, nil
	}

	// policyPreferActive. An explicitly preferred user always wants the JWT;
	// otherwise an API key wins without ever touching JWT validation.
	if strings.TrimSpace(tokens.PreferredCredential) != CredentialUserPreference && apiKey != "" {
		return Credential{Token: apiKey, Kind: CredentialAPIKey}, nil
	}

	// The JWT is the active candidate (preferred user, or no API key).
	if tokens.AccessToken == "" && tokens.RefreshToken == "" {
		return Credential{}, nil
	}
	userToken, err := t.resolveUserAccessToken(tokens)
	if err != nil {
		return Credential{}, breverrors.WrapAndTrace(err)
	}
	if userToken == "" {
		// Preferred user with an unusable JWT must not silently cross into
		// API-key mode; return nothing so the caller prompts for login.
		return Credential{}, nil
	}
	return Credential{Token: userToken, Kind: CredentialUserJWT}, nil
}

func (t Auth) ActivateUserCredential() error {
	return t.setPreferredCredential(CredentialUserPreference)
}

// setPreferredCredential persists which saved credential should authenticate requests
func (t Auth) setPreferredCredential(preference string) error {
	tokens, err := t.getSavedTokensOrNil()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if tokens == nil {
		tokens = &entity.AuthTokens{}
	}
	tokens.PreferredCredential = preference
	if err := t.authStore.SaveAuthTokens(*tokens); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// saveMergedTokens stores freshly-obtained JWT fields while preserving any
// saved API key (and its org) and the current preferred_credential
func (t Auth) saveMergedTokens(fresh entity.AuthTokens) error {
	existing, err := t.getSavedTokensOrNil()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if existing == nil {
		existing = &entity.AuthTokens{}
	}
	existing.AccessToken = fresh.AccessToken
	existing.RefreshToken = fresh.RefreshToken
	if err := t.authStore.SaveAuthTokens(*existing); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// resolveUserAccessToken returns a valid user access token from saved tokens,
// refreshing it when expired, or "" when no usable token exists.
func (t Auth) resolveUserAccessToken(tokens *entity.AuthTokens) (string, error) {
	if tokens.AccessToken == "" {
		breverrors.GetDefaultErrorReporter().ReportMessage("access token is an empty string but shouldn't be")
	}
	isAccessTokenValid, err := t.accessTokenValidator(tokens.AccessToken)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if !isAccessTokenValid {
		if tokens.RefreshToken == "" {
			// Expired with no way to refresh: no usable user token.
			return "", nil
		}
		tokens, err = t.getNewTokensWithRefreshOrNil(tokens.RefreshToken)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		if tokens == nil {
			return "", nil
		}
	}
	return tokens.AccessToken, nil
}

// Prompts for login and returns tokens, and saves to store
func (t Auth) PromptForLogin() (*LoginTokens, error) {
	shouldLogin, err := t.shouldLogin()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !shouldLogin {
		// Deliberately NOT wrapped, expected outcome
		return nil, &breverrors.DeclineToLoginError{}
	}

	tokens, err := t.Login(false)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return tokens, nil
}

func shouldLogin() (bool, error) {
	reader := bufio.NewReader(os.Stdin) // TODO 9 inject?
	fmt.Print(`You are currently logged out, would you like to log in? [Y/n]: `)
	text, err := reader.ReadString('\n')
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	trimmed := strings.ToLower(strings.TrimSpace(text))
	return trimmed == "y" || trimmed == "", nil
}

func (t Auth) LoginWithToken(token string) error {
	valid, err := isAccessTokenValid(token)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if valid {
		err = t.saveMergedTokens(entity.AuthTokens{
			AccessToken:  token,
			RefreshToken: "auto-login",
		})
	} else {
		err = t.saveMergedTokens(entity.AuthTokens{
			AccessToken:  "auto-login",
			RefreshToken: token,
		})
	}
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (t Auth) LoginWithAPIKey(apiKey string, orgID string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return breverrors.NewValidationError("api key is empty")
	}
	if !IsBrevAPIKey(apiKey) {
		return breverrors.NewValidationError(fmt.Sprintf("api key must start with %s", BrevAPIKeyPrefix))
	}

	tokens, err := t.getSavedTokensOrNil()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if tokens == nil {
		tokens = &entity.AuthTokens{}
	}
	tokens.APIKey = apiKey
	tokens.APIKeyOrgID = orgID
	tokens.PreferredCredential = CredentialAPIKeyPreference

	err = t.authStore.SaveAuthTokens(*tokens)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func init() {
	// pkg/browser pipes the launcher's stderr to ours, which leaks confusing
	// noise like "xdg-open: no DISPLAY environment variable specified" on
	// headless machines. We always print the login URL, so discard it.
	browser.Stderr = io.Discard
}

// showLoginURL prints the login link, framed as a fallback for when the
// browser doesn't open (e.g. headless machine, wrong default browser).
func showLoginURL(url string) {
	urlType := color.New(color.FgCyan, color.Bold).SprintFunc()
	fmt.Println("Browser didn't open? Use the URL below to sign in:")
	fmt.Println()
	fmt.Println(urlType(url))
}

func defaultAuthFunc(url, code string) {
	codeType := color.New(color.FgWhite, color.Bold).SprintFunc()
	if code != "" {
		fmt.Println("Your Device Confirmation Code is 👉", codeType(code), "👈")
		fmt.Print("\n")
	}

	// Best-effort: try to open the browser, but always show the URL below so
	// the user is never stranded if it doesn't open.
	_ = browser.OpenURL(url)
	showLoginURL(url)
	fmt.Println("\nWaiting for login to complete...")
}

func skipBrowserAuthFunc(url, _ string) {
	showLoginURL(url)
	fmt.Println("\nWaiting for login to complete...")
}

func (t Auth) Login(skipBrowser bool) (*LoginTokens, error) {
	authFunc := defaultAuthFunc
	if skipBrowser {
		authFunc = skipBrowserAuthFunc
	}
	tokens, err := t.oauth.DoDeviceAuthFlow(authFunc)
	if err != nil {
		fmt.Println("failed.")
		fmt.Println("")
		return nil, breverrors.WrapAndTrace(err)
	}

	// Merge the fresh JWT into the saved record so an API key credential is
	// preserved. Do NOT change preferred_credential here: only the explicit
	// login flow or a successful org switch may activate the user credential.
	if err := t.saveMergedTokens(tokens.AuthTokens); err != nil {
		fmt.Println("failed.")
		fmt.Println("")
		return nil, breverrors.WrapAndTrace(err)
	}

	caretType := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Println("")
	fmt.Println("  ", caretType("▸"), "   Successfully logged in.")

	return tokens, nil
}

func (t Auth) Logout() error {
	err := t.authStore.DeleteAuthTokens()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

type LoginTokens struct {
	entity.AuthTokens
	IDToken string
}

func (t Auth) getSavedTokensOrNil() (*entity.AuthTokens, error) {
	tokens, err := t.authStore.GetAuthTokens()
	if err != nil {
		switch err.(type) { //nolint:gocritic // like the ability to extend
		case *breverrors.CredentialsFileNotFound:
			return nil, nil
		}
		return nil, breverrors.WrapAndTrace(err)
	}
	if tokens != nil && tokens.AccessToken == "" && tokens.RefreshToken == "" && tokens.APIKey == "" {
		return nil, nil
	}
	return tokens, nil
}

// gets new access and refresh token or returns nil if refresh token expired, and updates store
func (t Auth) getNewTokensWithRefreshOrNil(refreshToken string) (*entity.AuthTokens, error) {
	tokens, err := t.oauth.GetNewAuthTokensWithRefresh(refreshToken)
	// TODO 2 handle if 403 invalid grant
	// https://stackoverflow.com/questions/57383523/how-to-detect-when-an-oauth2-refresh-token-expired
	if err != nil {
		if strings.Contains(err.Error(), "not implemented") {
			return nil, nil
		}
		return nil, breverrors.WrapAndTrace(err)
	}
	if tokens == nil {
		return nil, nil
	}
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = refreshToken
	}

	err = t.saveMergedTokens(*tokens)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return tokens, nil
}

func isAccessTokenValid(token string) (bool, error) {
	parser := jwt.Parser{}
	ptoken, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		// ValidationErrors occurred while parsing token is handled below. jwt.ValidationErrors is removed in new jwt v5
		if errors.Is(err, jwt.ErrTokenMalformed) || errors.Is(err, jwt.ErrTokenUnverifiable) {
			// fmt.Printf("warning: token error validation failed | %v\n", err)
			return false, nil
		}
		return false, breverrors.WrapAndTrace(err)
	}
	// Migrate from deprecated claims.Valid() to jwt v5 Validator.Validate() for standards-compliant claim validation.
	validator := jwt.NewValidator(
		jwt.WithIssuedAt(),
	)
	err = validator.Validate(ptoken.Claims)
	if err != nil {
		// https://pkg.go.dev/github.com/golang-jwt/jwt@v3.2.2+incompatible#MapClaims.Valid // https://github.com/dgrijalva/jwt-go/issues/383 // sometimes client clock is skew/out of sync with server who generated token
		if strings.Contains(err.Error(), "Token used before issued") { // not a security issue because we always check server side as well
			_ = 0
			// ignore error
		} else {
			// fmt.Printf("warning: token check validation failed | %v\n", err) // TODO need logger
			return false, nil
		}
	}
	return true, nil
}

func IssuerCheck(token string, issuer string) bool {
	parser := jwt.Parser{}
	claims := jwt.MapClaims{}
	_, _, err := parser.ParseUnverified(token, &claims)
	if err != nil {
		return false
	}
	iss, ok := claims["iss"].(string)
	if !ok {
		return false
	}
	return iss == issuer
}

func GetEmailFromToken(token string) string {
	parser := jwt.Parser{}
	claims := jwt.MapClaims{}
	_, _, err := parser.ParseUnverified(token, &claims)
	if err != nil {
		return ""
	}
	email, ok := claims["email"].(string)
	if !ok {
		return ""
	}
	return email
}

func AuthProviderFlagToCredentialProvider(authProviderFlag string) entity.CredentialProvider {
	if authProviderFlag == "" {
		return ""
	}
	if authProviderFlag == "nvidia" {
		return CredentialProviderKAS
	}
	return CredentialProviderAuth0
}

func StandardLogin(authProvider string, email string, tokens *entity.AuthTokens) OAuth {
	// Set KAS as the default authenticator
	shouldPromptEmail := false
	if email == "" && tokens != nil && tokens.AccessToken != "" && tokens.APIKey == "" {
		email = GetEmailFromToken(tokens.AccessToken)
		shouldPromptEmail = true
	}

	kasAuthenticator := NewKasAuthenticator(
		email,
		config.GlobalConfig.GetBrevAuthURL(),
		config.GlobalConfig.GetBrevAuthIssuerURL(),
		shouldPromptEmail,
		config.GlobalConfig.GetConsoleURL(),
	)

	// Create the auth0 authenticator as an alternative
	auth0Authenticator := Auth0Authenticator{
		Issuer:             "https://brevdev.us.auth0.com/",
		Audience:           "https://brevdev.us.auth0.com/api/v2/",
		ClientID:           "JaqJRLEsdat5w7Tb0WqmTxzIeqwqepmk",
		DeviceCodeEndpoint: "https://brevdev.us.auth0.com/oauth/device/code",
		OauthTokenEndpoint: "https://brevdev.us.auth0.com/oauth/token",
	}

	// Default to KAS authenticator
	var authenticator OAuth = kasAuthenticator

	authRetriever := NewOAuthRetriever([]OAuth{
		auth0Authenticator,
		kasAuthenticator,
	})

	if tokens != nil && tokens.AccessToken != "" && tokens.APIKey == "" {
		authenticatorFromToken, errr := authRetriever.GetByToken(tokens.AccessToken)
		if errr != nil {
			fmt.Printf("%v\n", errr)
		} else {
			authenticator = authenticatorFromToken
		}
	}

	if authProvider != "" {
		provider := AuthProviderFlagToCredentialProvider(authProvider)
		if provider == CredentialProviderAuth0 || provider == CredentialProviderKAS {
			oauth, errr := authRetriever.GetByProvider(provider)
			if errr != nil {
				fmt.Printf("%v\n", errr)
			} else {
				authenticator = oauth
			}
		}
	}

	return authenticator
}
