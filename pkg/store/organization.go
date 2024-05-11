package store

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

func (s AuthHTTPStore) SetDefaultOrganization(org *entity.Organization) error {
	home, err := s.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	path := files.GetActiveOrgsPath(home)

	err = files.OverwriteJSON(s.fs, path, org)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (f FileStore) ClearDefaultOrganization() error {
	home, err := f.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	path := files.GetActiveOrgsPath(home)
	err = files.DeleteFile(f.fs, path)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// returns the 'set'/active organization or nil if not set
func (s AuthHTTPStore) GetActiveOrganizationOrNil() (*entity.Organization, error) {
	workspaceID, err := s.GetCurrentWorkspaceID()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		var workspace *entity.Workspace
		workspace, err = s.GetWorkspace(workspaceID)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		var org *entity.Organization
		org, err = s.GetOrganization(workspace.OrganizationID)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		return org, nil
	}

	home, err := s.UserHomeDir()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	brevActiveOrgsFile := files.GetActiveOrgsPath(home)

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

	freshOrg, err := s.GetOrganization(activeOrg.ID)
	if err != nil {
		if !IsNetwork404Or403Error(err) { // handle because can login with bad cache
			return nil, breverrors.WrapAndTrace(err)
		}
	}
	return freshOrg, nil
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

var (
	orgParamName     = "organizationID"
	orgIDPathPattern = "api/organizations/%s"
	orgIDPath        = fmt.Sprintf(orgIDPathPattern, fmt.Sprintf("{%s}", orgParamName))
)

func (s AuthHTTPStore) GetOrganization(organizationID string) (*entity.Organization, error) {
	var result entity.Organization
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		SetPathParam(orgIDParamName, organizationID).
		Get(orgIDPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
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

func (s AuthHTTPStore) CreateInviteLink(organizationID string) (string, error) {
	var result string
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		Get(orgPath + "/" + organizationID + "/invite")
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return "", NewHTTPResponseError(res)
	}

	return result, nil
}

func GetDefaultOrNilOrg(orgs []entity.Organization) *entity.Organization {
	if len(orgs) > 0 {
		return &orgs[0]
	} else {
		return nil
	}
}
