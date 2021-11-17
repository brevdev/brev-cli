package store

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var mePath = "api/me"

func (s AuthHTTPStore) GetCurrentUser() (*brevapi.User, error) {
	var result brevapi.User
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		Get(mePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}

var userKeysPath = fmt.Sprintf("%s/keys", mePath)

func (s AuthHTTPStore) GetCurrentUserKeys() (*brevapi.UserKeys, error) {
	var result brevapi.UserKeys
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		Get(userKeysPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

var usersPath = "api/users"

func (n NoAuthHTTPStore) CreateUser(identityToken string) (*brevapi.User, error) {
	var result brevapi.User
	res, err := n.noAuthHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Identity", identityToken).
		SetResult(&result).
		Post(usersPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}
