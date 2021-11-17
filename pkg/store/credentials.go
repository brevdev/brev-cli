package store

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
)

func (f FileStore) WriteCredentialsToFile(credentials *brevapi.Credentials) error {
	brevCredentialsFile, err := files.GetCredentialFilePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = files.OverwriteJSON(*brevCredentialsFile, credentials)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (f FileStore) GetCredentialsFromFile() (*brevapi.Credentials, error) {
	brevCredentialsFile, err := files.GetCredentialFilePath()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	exists, err := files.Exists(*brevCredentialsFile, false)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !exists {
		return nil, &breverrors.CredentialsFileNotFound{}
	}

	var credentials brevapi.Credentials
	err = files.ReadJSON(f.fs, *brevCredentialsFile, &credentials)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &credentials, nil
}

func (f FileStore)  GetTokenFromBrevConfigFile() (*brevapi.OauthToken, error) {
	brevCredentialsFile, err := files.GetCredentialFilePath()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	exists, err := files.Exists(*brevCredentialsFile, false)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !exists {
		return nil, &breverrors.CredentialsFileNotFound{}
	}

	var token brevapi.OauthToken
	err = files.ReadJSON(files.AppFs, *brevCredentialsFile, &token)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &token, nil
}
