package store

import (
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

func (s AuthHTTPStore) SetDefaultOrganization(org *entity.Organization) error {
	path := files.GetActiveOrgsPath()

	err := files.OverwriteJSON(path, org)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

// returns the 'set'/active organization or nil if not set
func (s AuthHTTPStore) GetActiveOrganizationOrNil() (*entity.Organization, error) {
	brevActiveOrgsFile := files.GetActiveOrgsPath()
	exists, err := afero.Exists(s.fs, brevActiveOrgsFile)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !exists {
		return nil, nil
	}

	var activeOrg entity.Organization
	err = files.ReadJSON(s.fs, brevActiveOrgsFile, &activeOrg)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &activeOrg, nil
}

// returns the 'set'/active organization or the default one or nil if no orgs exist
func (s AuthHTTPStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	org, err := s.GetActiveOrganizationOrNil()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org != nil {
		return org, nil
	}

	orgs, err := s.GetOrganizations(nil)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return GetDefaultOrNilOrg(orgs), nil
}

var orgPath = "api/organizations"

type GetOrganizationsOptions struct {
	Name string
}

func (s AuthHTTPStore) GetOrganizations(options *GetOrganizationsOptions) ([]entity.Organization, error) {
	orgs, err := s.getOrganizations()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if options == nil || options.Name == "" {
		return orgs, nil
	}

	filteredOrgs := []entity.Organization{}
	for _, o := range orgs {
		if o.Name == options.Name {
			filteredOrgs = append(filteredOrgs, o)
		}
	}
	return filteredOrgs, nil
}

func (s AuthHTTPStore) getOrganizations() ([]entity.Organization, error) {
	var result []entity.Organization
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		Get(orgPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return result, nil
}

type CreateOrganizationRequest struct {
	Name string `json:"name"`
}

func (s AuthHTTPStore) CreateOrganization(req CreateOrganizationRequest) (*entity.Organization, error) {
	var result entity.Organization
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		SetBody(req).
		Post(orgPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}

func GetDefaultOrNilOrg(orgs []entity.Organization) *entity.Organization {
	if len(orgs) > 0 {
		return &orgs[0]
	} else {
		return nil
	}
}
