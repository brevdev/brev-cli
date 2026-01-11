package store

import (
	"encoding/json"

	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	resty "github.com/go-resty/resty/v2"
)

const (
	instanceTypesAPIURL  = "https://api.brev.dev"
	instanceTypesAPIPath = "v1/instance/types"
)

// GetInstanceTypes fetches all available instance types from the public API
func (s NoAuthHTTPStore) GetInstanceTypes() (*gpusearch.InstanceTypesResponse, error) {
	return fetchInstanceTypes()
}

// GetInstanceTypes fetches all available instance types from the public API
func (s AuthHTTPStore) GetInstanceTypes() (*gpusearch.InstanceTypesResponse, error) {
	return fetchInstanceTypes()
}

// fetchInstanceTypes fetches instance types from the public Brev API
func fetchInstanceTypes() (*gpusearch.InstanceTypesResponse, error) {
	client := resty.New()
	client.SetBaseURL(instanceTypesAPIURL)

	res, err := client.R().
		SetHeader("Accept", "application/json").
		Get(instanceTypesAPIPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	var result gpusearch.InstanceTypesResponse
	err = json.Unmarshal(res.Body(), &result)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &result, nil
}
