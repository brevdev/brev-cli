package store

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const (
	instanceTypesAPIPath = "v1/instance/types"
	// Authenticated API for instance types with workspace groups
	allInstanceTypesPathPattern = "api/instances/alltypesavailable/%s"
)

// GetInstanceTypes fetches all available instance types from the public API
func (s NoAuthHTTPStore) GetInstanceTypes(includeCPU bool) (*gpusearch.InstanceTypesResponse, error) {
	return fetchInstanceTypes(includeCPU)
}

// GetInstanceTypes fetches all available instance types from the public API
func (s AuthHTTPStore) GetInstanceTypes(includeCPU bool) (*gpusearch.InstanceTypesResponse, error) {
	return fetchInstanceTypes(includeCPU)
}

// fetchInstanceTypes fetches instance types from the public Brev API
func fetchInstanceTypes(includeCPU bool) (*gpusearch.InstanceTypesResponse, error) {
	cfg := config.NewConstants()
	client := NewRestyClient(cfg.GetBrevPublicAPIURL())

	req := client.R().
		SetHeader("Accept", "application/json")
	if includeCPU {
		req.SetQueryParam("include_cpu", "true")
	}
	res, err := req.Get(instanceTypesAPIPath)
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

// GetAllInstanceTypesWithWorkspaceGroups fetches instance types with workspace groups from the authenticated API
func (s AuthHTTPStore) GetAllInstanceTypesWithWorkspaceGroups(orgID string) (*gpusearch.AllInstanceTypesResponse, error) {
	path := fmt.Sprintf(allInstanceTypesPathPattern, orgID)

	var result gpusearch.AllInstanceTypesResponse
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParam("utm_source", "cli").
		SetQueryParam("os", runtime.GOOS).
		SetResult(&result).
		Get(path)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}
