package store

import breverrors "github.com/brevdev/brev-cli/pkg/errors"

const healthcheckPath = "api/health"

func (n NoAuthHTTPStore) Healthcheck() error {
	res, err := n.noAuthHTTPClient.restyClient.R().Get(healthcheckPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return NewHTTPResponseError(res)
	}

	return nil
}
