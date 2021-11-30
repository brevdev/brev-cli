package store

import (
	"os"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

// TODO 1 test cov

const brevCredentialsFile = "credentials.json"

func (f FileStore) SaveAuthTokens(token entity.AuthTokens) error {
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

func (f FileStore) GetAuthTokens() (*entity.AuthTokens, error) {
	brevCredentialsFile, err := getBrevCredentialsFile()
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
	return &token, nil
}

func getBrevCredentialsFile() (*string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	brevCredentialsFile := home + "/" + files.GetBrevDirectory() + "/" + brevCredentialsFile
	return &brevCredentialsFile, nil
}

func (f FileStore) DeleteAuthTokens() error {
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
