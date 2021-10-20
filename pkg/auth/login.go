package auth

import (
	"os"

	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

const (
	brevCredentialsFile      = "credentials.json"
	globalActiveProjectsFile = "active_projects.json"
)

type OauthToken struct {
	AccessToken  string `json:"access_token"`
	AuthMethod   string `json:"auth_method"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
}

func initializeActiveProjectsFile(t *terminal.Terminal) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	brevActiveProjectsFile := home + "/" + files.GetBrevDirectory() + "/" + globalActiveProjectsFile
	exists, err := files.Exists(brevActiveProjectsFile, false)
	if err != nil {
		return err
	}
	if !exists {
		err = files.OverwriteJSON(brevActiveProjectsFile, []string{})
		if err != nil {
			t.Errprint(err, "Failed to initialize active projects file. Just run this and try again: echo '[]' > ~/.brev/active_projects.json ")
			return err
		}
	}

	return nil
}

// GetToken reads the previously-persisted token from the filesystem but may issue a round
// trip request to Cotter if the token is determined to have expired:
//   1. Read the Cotter token from the hidden brev directory
//   2. Determine if the token is valid
//   3. If valid, return
//   4. If invalid, issue a refresh request to Cotter
//   5. Write the refreshed Cotter token to a file in the hidden brev directory
func GetToken() (*OauthToken, error) {
	token, err := getTokenFromBrevConfigFile()
	if err != nil {
		return nil, err
	}
	return token, nil
}

type Credentials struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
}

func WriteTokenToBrevConfigFile(token *Credentials) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	brevCredentialsFile := home + "/" + files.GetBrevDirectory() + "/" + brevCredentialsFile

	err = files.OverwriteJSON(brevCredentialsFile, token)
	if err != nil {
		return err
	}

	return nil
}

func getTokenFromBrevConfigFile() (*OauthToken, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	brevCredentialsFile := home + "/" + files.GetBrevDirectory() + "/" + brevCredentialsFile
	exists, err := files.Exists(brevCredentialsFile, false)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, &brev_errors.CredentialsFileNotFound{}
	}

	var token OauthToken
	err = files.ReadJSON(brevCredentialsFile, &token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}
