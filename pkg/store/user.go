package store

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var userPath = "api/me"

func (s AuthHTTPStore) GetCurrentUser() (*brevapi.User, error) {
	var result brevapi.User
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		Get(userPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}

var userKeysPath = fmt.Sprintf("%s/keys", userPath)

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
