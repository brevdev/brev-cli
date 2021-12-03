package auth

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
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
}

type Auth struct {
	authStore AuthStore
	oauth     OAuth
}

func NewAuth(authStore AuthStore, oauth OAuth) *Auth {
	return &Auth{
		authStore: authStore,
		oauth:     oauth,
	}
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
	isAccessTokenValid, err := isAccessTokenValid(tokens.AccessToken)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if !isAccessTokenValid {
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
	reader := bufio.NewReader(os.Stdin) // TODO 9 inject?
	fmt.Print(`You are currently logged out, would you like to log in? [y/n]: `)
	text, _ := reader.ReadString('\n')
	if strings.Compare(text, "y") != 1 {
		return nil, &breverrors.DeclineToLoginError{}
	}

	tokens, err := t.Login()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return tokens, nil
}

func (t Auth) Login() (*LoginTokens, error) {
	tokens, err := t.oauth.DoDeviceAuthFlow(
		func(url, code string) {
			fmt.Println("Your Device Confirmation Code is", code)

			err := browser.OpenURL(url)
			if err != nil {
				fmt.Println("please open: ", url)
			}

			fmt.Println("waiting for auth to complete")
		},
	)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	err = t.authStore.SaveAuthTokens(tokens.AuthTokens)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	fmt.Print("\n")
	fmt.Println("Successfully logged in.")

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
	if tokens != nil && tokens.AccessToken == "" {
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
			return false, nil
		}
		return false, breverrors.WrapAndTrace(err)
	}
	err = ptoken.Claims.Valid()
	if err != nil {
		return false, nil
	}
	return true, nil
}
