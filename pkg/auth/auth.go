package auth

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/fatih/color"
	"github.com/golang-jwt/jwt"
	"github.com/pkg/browser"
)

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

	// should always at least have access token?
	if tokens.AccessToken == "" {
		breverrors.GetDefaultErrorReporter().ReportMessage("access token is an empty string but shouldn't be")
	}
	isAccessTokenValid, err := t.accessTokenValidator(tokens.AccessToken)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if !isAccessTokenValid && tokens.RefreshToken != "" {
		tokens, err = t.getNewTokensWithRefreshOrNil(tokens.RefreshToken)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		if tokens == nil {
			return "", nil
		}
	} else if tokens.RefreshToken == "" && tokens.AccessToken == "" {
		return "", nil
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
	fmt.Print(`You are currently logged out, would you like to log in? [y/n]: `)
	text, err := reader.ReadString('\n')
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	return strings.ToLower(strings.TrimSpace(text)) == "y", nil
}

func (t Auth) LoginWithToken(token string) error {
	valid, err := isAccessTokenValid(token)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if valid {
		err := t.authStore.SaveAuthTokens(entity.AuthTokens{
			AccessToken:  token,
			RefreshToken: "auto-login",
		})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		err := t.authStore.SaveAuthTokens(entity.AuthTokens{
			AccessToken:  "auto-login",
			RefreshToken: token,
		})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func defaultAuthFunc(url, code string) {
	codeType := color.New(color.FgWhite, color.Bold).SprintFunc()
	if code != "" {
		fmt.Println("Your Device Confirmation Code is 👉", codeType(code), "👈")
		fmt.Print("\n")
	}
	urlType := color.New(color.FgCyan, color.Bold).SprintFunc()
	fmt.Println("Browser link: " + urlType(url) + "\n")
	fmt.Println("Alternatively, get CLI Command (\"Login via CLI\"): ", urlType(fmt.Sprintf("%s/profile?login=cli", config.ConsoleBaseURL)))
	fmt.Print("\n")
	caretType := color.New(color.FgGreen, color.Bold).SprintFunc()
	enterType := color.New(color.FgGreen, color.Bold).SprintFunc()
	_ = terminal.PromptGetInput(terminal.PromptContent{
		Label:      "   " + caretType("▸") + "    Press " + enterType("Enter") + " to login via browser",
		ErrorMsg:   "error",
		AllowEmpty: true,
	})

	fmt.Print("\n")

	err := browser.OpenURL(url)
	if err != nil {
		fmt.Println("Error opening browser. Please copy", urlType(url), "and paste it in your browser.")
	}
	fmt.Println("Waiting for login to complete in browser... ")
}

func skipBrowserAuthFunc(url, _ string) {
	urlType := color.New(color.FgCyan, color.Bold).SprintFunc()
	fmt.Println("Please copy", urlType(url), "and paste it in your browser.")
	fmt.Println("Alternatively, get CLI Command (\"Login via CLI\"): ", urlType(fmt.Sprintf("%s/profile?login=cli", config.ConsoleBaseURL)))
	fmt.Println("Waiting for login to complete in browser... Ctrl+C to use CLI command instead.")
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
		ve := &jwt.ValidationError{}
		if errors.As(err, &ve) {
			// fmt.Printf("warning: token error validation failed | %v\n", err)
			return false, nil
		}
		return false, breverrors.WrapAndTrace(err)
	}
	err = ptoken.Claims.Valid()
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
		"https://api.ngc.nvidia.com",
		"https://login.nvidia.com",
		shouldPromptEmail,
		"https://brev.nvidia.com",
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

	if authenticator.GetCredentialProvider() == CredentialProviderKAS {
		config.ConsoleBaseURL = "https://brev.nvidia.com"
	}

	return authenticator
}
