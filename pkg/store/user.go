package store

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func (s AuthHTTPStore) GetCurrentUser() (*brevapi.User, error) {
	user, err := s.authHTTPClient.toDeprecateClient.GetMe() // todo check cache first
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return user, nil
}

func (s AuthHTTPStore) GetCurrentUserKeys() (*brevapi.UserKeys, error) {
	keys, err := s.authHTTPClient.toDeprecateClient.GetMeKeys()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return keys, nil
}
