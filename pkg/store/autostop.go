package store

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const pathRegisterNotificationEmail = "api/autostop/register"

func (n NoAuthHTTPStore) RegisterNotificationEmail(email string) error {
	var result UserCreateResponse
	res, err := n.noAuthHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		SetBody(map[string]any{
			email: email,
		}).
		Post(pathRegisterNotificationEmail)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return NewHTTPResponseError(res)
	}

	return nil
}
