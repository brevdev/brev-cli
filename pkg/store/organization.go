package store

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

// returns the 'set'/active organization or nil if not set
func (s AuthHTTPStore) GetActiveOrganizationOrNil() (*brevapi.Organization, error) {
	brevActiveOrgsFile := files.GetActiveOrgsPath()
	exists, err := afero.Exists(s.fs, brevActiveOrgsFile)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !exists {
		return nil, nil
	}

	var activeOrg brevapi.Organization
	err = files.ReadJSON(s.fs, brevActiveOrgsFile, &activeOrg)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &activeOrg, nil
}

// returns the 'set'/active organization or the default one or nil if no orgs exist
func (s AuthHTTPStore) GetActiveOrganizationOrDefault() (*brevapi.Organization, error) {
	org, err := s.GetActiveOrganizationOrNil()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org != nil {
		return org, nil
	}

	orgs, err := s.GetOrganizations()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return GetDefaultOrNilOrg(orgs), nil
}

var orgPath = "api/organizations"

func (s AuthHTTPStore) GetOrganizations() ([]brevapi.Organization, error) {
	var result []brevapi.Organization
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		// ForceContentType("application/json").
		SetResult(&result).
		Get(orgPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.StatusCode() > 299 {
	}

	return result, nil
}

func GetDefaultOrNilOrg(orgs []brevapi.Organization) *brevapi.Organization {
	if len(orgs) > 0 {
		return &orgs[0]
	} else {
		return nil
	}
}
