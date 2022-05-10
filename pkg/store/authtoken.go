package store

import (
	"fmt"
	"io/ioutil"
	"path"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

// TODO 1 test cov

const (
	brevCredentialsFile = "credentials.json"
	brevDirectory       = ".brev"
)

func GetBrevDirectory() string {
	return brevDirectory
}

func (f FileStore) SaveAuthTokens(token entity.AuthTokens) error {
	if token.AccessToken == "" {
		return fmt.Errorf("access token is empty")
	}
	brevCredentialsFile, err := f.getBrevCredentialsFile()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Printf("token save: %v\nat %s\n", token, *brevCredentialsFile)
	err = files.OverwriteJSON(f.fs, *brevCredentialsFile, token)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (f FileStore) GetAuthTokens() (*entity.AuthTokens, error) {
	workspaceID, err := f.GetCurrentWorkspaceID()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		var serviceToken string
		serviceToken, err = f.GetCurrentWorkspaceServiceToken()
		if err == nil {
			return &entity.AuthTokens{
				AccessToken: serviceToken,
			}, nil
		} else {
			// log that getting service account token failed
			_ = 0 // noop to make linter happy
		}
	}

	brevCredentialsFile, err := f.getBrevCredentialsFile()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	exists, err := afero.Exists(f.fs, *brevCredentialsFile)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !exists {
		return nil, &breverrors.CredentialsFileNotFound{}
	}

	var token entity.AuthTokens
	err = files.ReadJSON(f.fs, *brevCredentialsFile, &token)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	fmt.Printf("token get access: %s | refresh %s\n", token.AccessToken, token.RefreshToken)
	return &token, nil
}

func (f FileStore) GetCurrentWorkspaceServiceToken() (string, error) {
	saTokenFilePath := getServiceTokenFilePath()
	saTokenFile, err := f.fs.Open(saTokenFilePath)
	defer saTokenFile.Close() //nolint: errcheck // defer is fine
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	token, err := ioutil.ReadAll(saTokenFile)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return string(token), nil
}

func getServiceTokenFilePath() string {
	return "/var/run/secrets/kubernetes.io/serviceaccount/token"
}

func (f FileStore) DeleteAuthTokens() error {
	brevCredentialsFile, err := f.getBrevCredentialsFile()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = files.DeleteFile(f.fs, *brevCredentialsFile)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) getBrevCredentialsFile() (*string, error) {
	home, err := f.UserHomeDir()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	brevCredentialsFile := path.Join(home, brevDirectory, brevCredentialsFile)
	return &brevCredentialsFile, nil
}
