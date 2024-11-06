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

	breverrors.GetDefaultErrorReporter().SetUser(breverrors.ErrorUser{
		ID:       result.ID,
		Username: result.Username,
		Email:    result.Email,
	})

	return &result, nil
}

func (s AuthHTTPStore) GetCurrentUserID() (string, error) {
	meta, err := s.GetCurrentWorkspaceMeta()
	if err != nil {
		return "", nil
	}
	if meta.UserID != "" {
		return meta.UserID, nil
	}
	user, err := s.GetCurrentUser()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return user.ID, nil
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

type UserCreateResponse struct {
	User         entity.User `json:"user"`
	ErrorMessage string      `json:"errorMessage"`
}

func (n NoAuthHTTPStore) CreateUser(identityToken string) (*entity.User, error) {
	var result UserCreateResponse
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

	return &result.User, nil
}

func (s AuthHTTPStore) UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error) {
	var result entity.User
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		SetBody(updatedUser).
		// SetPathParam(userIDParamName, userID).
		Put(usersPath + "/" + userID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}

// 	userIDParamName = "userID"
// 	userIDParamStr  = fmt.Sprintf("{%s}", userIDParamName)

var usersIDPathPattern = fmt.Sprintf("%s/%s", usersPath, "%s")

// usersIDPath        = fmt.Sprintf(usersIDPathPattern, fmt.Sprintf("{%s}", userIDParamStr))

func (s AuthHTTPStore) GetUsers(queryParams map[string]string) ([]entity.User, error) {
	var result []entity.User
	res, err := s.authHTTPClient.restyClient.R().
		SetQueryParams(queryParams).
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		Get(usersPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return result, nil
}
