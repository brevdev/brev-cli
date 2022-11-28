package store

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const pathDownloadUrl = "api/autostop/cli-download-url"

func (n NoAuthHTTPStore) DownloadUrl() (string, error) {
	res, err := n.noAuthHTTPClient.restyClient.R().
		Get(pathDownloadUrl)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return "", NewHTTPResponseError(res)
	}
	return res.String(), nil
}

const pathRegisterNotificationEmail = "api/autostop/register"

func (n NoAuthHTTPStore) RegisterNotificationEmail(email string) error {
	res, err := n.noAuthHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]any{
			"email": email,
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

const pathRecordAutoStop = "api/autostop/record"

type RecordAutopstopBody struct {
	Email        string
	InstanceType string
	Region       string
	Name         string
	EnvID        string
}

func (n NoAuthHTTPStore) RecordAutoStop(
	recordAutopstopBody RecordAutopstopBody,
) error {
	res, err := n.noAuthHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(recordAutopstopBody).
		Post(pathRecordAutoStop)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return NewHTTPResponseError(res)
	}

	return nil
}
