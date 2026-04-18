package auth

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/fatih/color"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/browser"
)

// refreshBeforeExpiry is how far in advance of access-token expiration the
// CLI refreshes. Using a window larger than typical request RTTs avoids 401
// round-trips at the tail of a token's life, at the cost of refreshing a
// small number of still-valid tokens.
const refreshBeforeExpiry = 5 * time.Minute

type LoginAuth struct {
	Auth
}

func NewLoginAuth(authStore AuthStore, oauth OAuth) *LoginAuth {
	return &LoginAuth{
		Auth: *NewAuth(authStore, oauth),
	}
}

func (l LoginAuth) GetAccessToken() (string, error) {
	token, err := l.GetFreshAccessTokenOrLogin()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return token, nil
}

type NoLoginAuth struct {
	Auth
}

func NewNoLoginAuth(authStore AuthStore, oauth OAuth) *NoLoginAuth {
	return &NoLoginAuth{
		Auth: *NewAuth(authStore, oauth),
	}
}

func (l NoLoginAuth) GetAccessToken() (string, error) {
	token, err := l.GetFreshAccessTokenOrNil()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return token, nil
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
	tokens, err := t.getSavedTokensOrNil()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if tokens == nil {
		return "", nil
	}

	// Older CLI versions stored the literal string "auto-login" when
	// `brev login --token` had no real refresh token to save. Treat it as
	// absent so we do not attempt to exchange it with the IdP and fail.
	if tokens.RefreshToken == autoLoginSentinel {
		tokens.RefreshToken = ""
	}

	// should always at least have access token?
	if tokens.AccessToken == "" {
		breverrors.GetDefaultErrorReporter().ReportMessage("access token is an empty string but shouldn't be")
	}
	isAccessTokenValid, err := t.accessTokenValidator(tokens.AccessToken)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	// Trigger a refresh when the token is invalid OR when it is still valid
	// but close enough to expiry that the next API call is likely to race
	// the exp boundary. The proactive branch is tolerant of refresh failure:
	// if the IdP is briefly unreachable, fall back to the (still-valid)
	// current access token rather than logging the user out.
	expiringSoon := isAccessTokenValid && tokens.RefreshToken != "" && accessTokenExpiresSoon(tokens)
	if !isAccessTokenValid || expiringSoon {
		if tokens.RefreshToken == "" {
			// Access token is expired and we have no refresh token. Returning
			// the expired token here would just cause a 401 on the next API
			// call; return empty so callers can prompt for re-login instead.
			return "", nil
		}
		newTokens, refreshErr := t.getNewTokensWithRefreshOrNil(tokens.RefreshToken)
		if refreshErr != nil {
			if expiringSoon {
				// Current token still validates; swallow the transient
				// failure and try again on the next call.
				return tokens.AccessToken, nil
			}
			return "", breverrors.WrapAndTrace(refreshErr)
		}
		if newTokens == nil {
			return "", nil
		}
		tokens = newTokens
	}
	return tokens.AccessToken, nil
}

// accessTokenExpiresSoon reports whether the stored access token's
// expiration is within refreshBeforeExpiry of now. It prefers the persisted
// AccessTokenExp field (written by populateTokenTimestamps on save) and
// falls back to decoding the access JWT for files written by older CLI
// versions that never persisted the claim.
func accessTokenExpiresSoon(tokens *entity.AuthTokens) bool {
	var exp time.Time
	if tokens.AccessTokenExp != nil {
		exp = *tokens.AccessTokenExp
	} else {
		exp, _ = accessTokenClaims(tokens.AccessToken)
	}
	if exp.IsZero() {
		return false
	}
	return time.Until(exp) < refreshBeforeExpiry
}

// accessTokenClaims parses the access JWT without signature verification
// and returns its exp and iat claims. Missing or malformed claims are
// returned as the zero time.Time; the caller is responsible for guarding
// with IsZero().
func accessTokenClaims(token string) (exp, iat time.Time) {
	if token == "" {
		return time.Time{}, time.Time{}
	}
	parser := jwt.Parser{}
	ptoken, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return time.Time{}, time.Time{}
	}
	claims, ok := ptoken.Claims.(jwt.MapClaims)
	if !ok {
		return time.Time{}, time.Time{}
	}
	if v, ok := claims["exp"].(float64); ok {
		exp = time.Unix(int64(v), 0)
	}
	if v, ok := claims["iat"].(float64); ok {
		iat = time.Unix(int64(v), 0)
	}
	return exp, iat
}

// populateTokenTimestamps fills in AccessTokenExp and IssuedAt from the
// access JWT when they are not already set. Safe to call on any AuthTokens
// value; missing or non-JWT access tokens leave the fields nil.
func populateTokenTimestamps(tokens *entity.AuthTokens) {
	if tokens == nil || tokens.AccessToken == "" {
		return
	}
	exp, iat := accessTokenClaims(tokens.AccessToken)
	if tokens.AccessTokenExp == nil && !exp.IsZero() {
		tokens.AccessTokenExp = &exp
	}
	if tokens.IssuedAt == nil && !iat.IsZero() {
		tokens.IssuedAt = &iat
	}
}

// isTransientRefreshError reports whether an error from the OAuth refresh
// call is a transient network condition (timeout, connection refused,
// DNS failure, etc.) as opposed to an authoritative rejection of the
// refresh token by the IdP. Transient errors should not force the user to
// re-login.
func isTransientRefreshError(err error) bool {
	if err == nil {
		return false
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return true
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// DNS / connection-refused / TLS handshake errors surface as url.Error
	// wrapping an *net.OpError. Treat connection-level failures as
	// transient: the refresh token is probably fine, the network isn't.
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

// Prompts for login and returns tokens, and saves to store
func (t Auth) PromptForLogin() (*LoginTokens, error) {
	shouldLogin, err := t.shouldLogin()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !shouldLogin {
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

// autoLoginSentinel is a legacy value older CLI versions stored in place of
// a real token when `brev login --token` was used. It is not a valid token of
// any kind; treat it as "absent" on read.
const autoLoginSentinel = "auto-login"

func (t Auth) LoginWithToken(token string) error {
	valid, err := isAccessTokenValid(token)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if valid {
		// The token is a self-contained JWT access token with no accompanying
		// refresh token. Previously we stored the string "auto-login" in the
		// RefreshToken slot; when the access token expired the refresh path
		// then attempted to exchange that sentinel with the IdP, which always
		// failed, logging the user out every time the short-lived access
		// token aged out. Store an empty RefreshToken instead so the refresh
		// path correctly recognizes there is nothing to refresh and prompts
		// for a fresh login exactly once.
		fmt.Fprintln(os.Stderr, "Note: tokens from --token cannot be refreshed; re-run `brev login` when the session expires.")
		tokens := entity.AuthTokens{
			AccessToken:  token,
			RefreshToken: "",
		}
		populateTokenTimestamps(&tokens)
		if err := t.authStore.SaveAuthTokens(tokens); err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		// The token is not a JWT, assume it is a refresh token. The access
		// token slot is filled with the sentinel so the first API call
		// triggers a refresh to populate a real access token.
		if err := t.authStore.SaveAuthTokens(entity.AuthTokens{
			AccessToken:  autoLoginSentinel,
			RefreshToken: token,
		}); err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

// showLoginURL displays the login link and CLI alternative for manual navigation.
func showLoginURL(url string) {
	urlType := color.New(color.FgCyan, color.Bold).SprintFunc()
	fmt.Println("Login here: " + urlType(url))
}

func defaultAuthFunc(url, code string) {
	codeType := color.New(color.FgWhite, color.Bold).SprintFunc()
	if code != "" {
		fmt.Println("Your Device Confirmation Code is 👉", codeType(code), "👈")
		fmt.Print("\n")
	}

	if hasBrowser() {
		if err := browser.OpenURL(url); err == nil {
			fmt.Println("Waiting for login to complete in browser...")
			return
		}
	}
	showLoginURL(url)
	fmt.Println("\nWaiting for login to complete...")
}

func skipBrowserAuthFunc(url, _ string) {
	showLoginURL(url)
	fmt.Println("\nWaiting for login to complete...")
}

// hasBrowser reports whether a browser can be opened on the current platform.
func hasBrowser() bool {
	if runtime.GOOS == "darwin" {
		// macOS always has "open".
		return true
	}
	// Linux: check for a known browser launcher.
	for _, name := range []string{"xdg-open", "x-www-browser", "www-browser"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
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

	err = t.authStore.SaveAuthTokens(tokens.AuthTokens)
	if err != nil {
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
	if tokens != nil && tokens.AccessToken == "" && tokens.RefreshToken == "" {
		return nil, nil
	}
	return tokens, nil
}

// gets new access and refresh token or returns nil if refresh token expired, and updates store
func (t Auth) getNewTokensWithRefreshOrNil(refreshToken string) (*entity.AuthTokens, error) {
	tokens, err := t.oauth.GetNewAuthTokensWithRefresh(refreshToken)
	if err != nil {
		if strings.Contains(err.Error(), "not implemented") {
			return nil, nil
		}
		if isTransientRefreshError(err) {
			// Network hiccup; do not clear the user's session. Surface the
			// error so the caller can decide whether to swallow it (when
			// the current access token is still valid) or propagate it.
			return nil, breverrors.WrapAndTrace(fmt.Errorf("could not reach auth provider to refresh session: %w", err))
		}
		// Definitive rejection from the IdP. Tell the user in plain
		// language rather than burying it in a stack trace.
		fmt.Fprintln(os.Stderr, "Your brev session could not be refreshed; re-run `brev login`.")
		return nil, nil
	}
	if tokens == nil {
		return nil, nil
	}
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = refreshToken
	}
	populateTokenTimestamps(tokens)

	err = t.authStore.SaveAuthTokens(*tokens)
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
	if email == "" && tokens != nil && tokens.AccessToken != "" {
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

	if tokens != nil && tokens.AccessToken != "" {
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
