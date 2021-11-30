package deprecatedauth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/auth"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/pkg/browser"
	"github.com/spf13/afero"
)

type Credentials struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
}

type OauthToken struct {
	AccessToken  string `json:"access_token"`
	AuthMethod   string `json:"auth_method"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
}

type TempAuth struct{}

func (t TempAuth) GetAccessToken() (string, error) {
	return GetAccessToken()
}

const brevCredentialsFile = "credentials.json"

func GetAccessToken() (string, error) {
	oauthToken, err := GetToken()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	return oauthToken.AccessToken, nil
}

func getBrevCredentialsFile() (*string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	brevCredentialsFile := home + "/" + files.GetBrevDirectory() + "/" + brevCredentialsFile
	return &brevCredentialsFile, nil
}

func WriteTokenToBrevConfigFile(token *Credentials) error {
	brevCredentialsFile, err := getBrevCredentialsFile()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = files.OverwriteJSON(*brevCredentialsFile, token)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func GetTokenFromBrevConfigFile(fs afero.Fs) (*OauthToken, error) {
	brevCredentialsFile, err := getBrevCredentialsFile()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	exists, err := afero.Exists(fs, *brevCredentialsFile)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !exists {
		return nil, &breverrors.CredentialsFileNotFound{}
	}

	var token OauthToken
	err = files.ReadJSON(fs, *brevCredentialsFile, &token)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &token, nil
}

func Login(prompt bool) (*string, error) {
	if prompt {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(`You are currently logged out, would you like to log in? [y/n]: `)
		text, _ := reader.ReadString('\n')
		if strings.Compare(text, "y") != 1 {
			return nil, &breverrors.DeclineToLoginError{}
		}
	}
	ctx := context.Background()

	// TODO env vars
	authenticator := auth.Authenticator{
		Audience:           "https://brevdev.us.auth0.com/api/v2/",
		ClientID:           "JaqJRLEsdat5w7Tb0WqmTxzIeqwqepmk",
		DeviceCodeEndpoint: "https://brevdev.us.auth0.com/oauth/device/code",
		OauthTokenEndpoint: "https://brevdev.us.auth0.com/oauth/token",
	}
	state, err := authenticator.Start(ctx)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err, "could not start the authentication process")
	}

	// todo color library
	// fmt.Printf("Your Device Confirmation code is: %s\n\n", ansi.Bold(state.UserCode))
	// cli.renderer.Infof("%s to open the browser to log in or %s to quit...", ansi.Green("Press Enter"), ansi.Red("^C"))
	// fmt.Scanln()
	// TODO make this stand out! its important
	fmt.Println("Your Device Confirmation Code is", state.UserCode)

	err = browser.OpenURL(state.VerificationURI)

	if err != nil {
		fmt.Println("please open: ", state.VerificationURI)
	}

	fmt.Println("waiting for auth to complete")

	res, err := authenticator.Wait(ctx, state)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err, "login error")
	}

	fmt.Print("\n")
	fmt.Println("Successfully logged in.")
	creds := &Credentials{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresIn:    int(res.ExpiresIn),
		IDToken:      res.IDToken,
	}
	// store the refresh token
	err = WriteTokenToBrevConfigFile(creds)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// hydrate the cache
	// _, _, err = WriteCaches()
	// if err != nil {
	// 	return nil, breverrors.WrapAndTrace(err)
	// }

	return &creds.IDToken, nil
}

func Logout() error {
	brevCredentialsFile, err := getBrevCredentialsFile()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = files.DeleteFile(*brevCredentialsFile)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// GetToken reads the previously-persisted token from the filesystem,
// returning nil for a token if it does not exist
func GetToken() (*OauthToken, error) {
	token, err := GetTokenFromBrevConfigFile(files.AppFs)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if token == nil { // we have not logged in yet
		_, err = Login(true)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		// now that we have logged in, the file should contain the token
		token, err = GetTokenFromBrevConfigFile(files.AppFs)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}
	return token, nil
}
