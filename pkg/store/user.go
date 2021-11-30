package store

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var mePath = "api/me"

func (s AuthHTTPStore) GetCurrentUser() (*entity.User, error) {
	var result entity.User
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

func (s AuthHTTPStore) GetCurrentUserKeys() (*entity.UserKeys, error) {
	var result entity.UserKeys
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

func (n NoAuthHTTPStore) CreateUser(identityToken string) (*entity.User, error) {
	var result entity.User
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
