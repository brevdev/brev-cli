package store

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplaneapiv1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/config"
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
	if err != nil && !strings.Contains(err.Error(), "no such file or directory") {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) GetCachedActiveOrganizationOrNil() (*entity.Organization, error) {
	home, err := f.UserHomeDir()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	brevActiveOrgsFile := files.GetActiveOrgsPath(home)

	exists, err := afero.Exists(f.fs, brevActiveOrgsFile)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !exists {
		return nil, nil
	}

	var activeOrg entity.Organization
	err = files.ReadJSON(f.fs, brevActiveOrgsFile, &activeOrg)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &activeOrg, nil
}

// returns the 'set'/active organization or nil if not set
func (s AuthHTTPStore) GetActiveOrganizationOrNil() (*entity.Organization, error) {
	if auth.IsAPIKeyAuthStore(&s) {
		orgID, err := auth.GetAPIKeyOrgID(&s)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		org := &entity.Organization{ID: orgID, Name: orgID}
		// Name hydration is best-effort; the command itself should surface backend auth errors.
		freshOrg, err := s.GetOrganization(orgID)
		if err != nil {
			return org, nil
		}
		if freshOrg == nil {
			return org, nil
		}
		if freshOrg.ID == "" {
			freshOrg.ID = orgID
		}
		if freshOrg.Name == "" {
			freshOrg.Name = freshOrg.ID
		}
		return freshOrg, nil
	}

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

	activeOrg, err := s.GetCachedActiveOrganizationOrNil()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if activeOrg == nil {
		return nil, nil
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
		if strings.EqualFold(o.Name, options.Name) {
			filteredOrgs = append(filteredOrgs, o)
		}
	}
	return filteredOrgs, nil
}

// GetOrganizationsByName returns organizations matching the given name.
func (s AuthHTTPStore) GetOrganizationsByName(name string) ([]entity.Organization, error) {
	return s.GetOrganizations(&GetOrganizationsOptions{Name: name})
}

// ListOrganizations returns all organizations (for prompt-driven register flow).
func (s AuthHTTPStore) ListOrganizations() ([]entity.Organization, error) {
	return s.GetOrganizations(nil)
}

func (s AuthHTTPStore) getOrganizations() ([]entity.Organization, error) {
	res, err := s.organizationServiceClient().ListOrganization(
		context.Background(),
		connect.NewRequest(&devplaneapiv1.ListOrganizationRequest{}),
	)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return mapDevplaneOrganizations(res.Msg.GetItems()), nil
}

func (s AuthHTTPStore) GetOrganization(organizationID string) (*entity.Organization, error) {
	res, err := s.organizationServiceClient().GetOrganization(
		context.Background(),
		connect.NewRequest(&devplaneapiv1.GetOrganizationRequest{
			OrganizationId: organizationID,
		}),
	)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	org := mapDevplaneOrganization(res.Msg.GetOrganization())
	return &org, nil
}

type CreateOrganizationRequest struct {
	Name string `json:"name"`
}

func (s AuthHTTPStore) CreateOrganization(req CreateOrganizationRequest) (*entity.Organization, error) {
	res, err := s.organizationServiceClient().CreateOrganization(
		context.Background(),
		connect.NewRequest(&devplaneapiv1.CreateOrganizationRequest{
			DisplayName: req.Name,
		}),
	)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	org := mapDevplaneOrganization(res.Msg.GetOrganization())
	return &org, nil
}

func (s AuthHTTPStore) organizationServiceClient() devplaneapiv1connect.OrganizationServiceClient {
	return devplaneapiv1connect.NewOrganizationServiceClient(
		newAuthenticatedConnectHTTPClient(s.authHTTPClient.auth),
		config.GlobalConfig.GetBrevPublicAPIURL(),
	)
}

type connectBearerTokenTransport struct {
	auth Auth
	base http.RoundTripper
}

func (t connectBearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.auth.GetAccessToken()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return resp, nil
}

func newAuthenticatedConnectHTTPClient(auth Auth) *http.Client {
	return &http.Client{
		Transport: connectBearerTokenTransport{
			auth: auth,
			base: http.DefaultTransport,
		},
	}
}

func mapDevplaneOrganizations(orgs []*devplaneapiv1.Organization) []entity.Organization {
	result := make([]entity.Organization, 0, len(orgs))
	for _, org := range orgs {
		result = append(result, mapDevplaneOrganization(org))
	}
	return result
}

func mapDevplaneOrganization(org *devplaneapiv1.Organization) entity.Organization {
	if org == nil {
		return entity.Organization{}
	}
	name := org.GetDisplayName()
	if name == "" {
		name = org.GetUsername()
	}
	if name == "" {
		name = org.GetOrganizationId()
	}
	return entity.Organization{
		ID:   org.GetOrganizationId(),
		Name: name,
	}
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

func (s AuthHTTPStore) GetOrgRoleAttachments(orgID string) ([]entity.OrgRoleAttachment, error) {
	var result []entity.OrgRoleAttachment
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		Get(fmt.Sprintf("api/organizations/%s/role_attachments", orgID))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return result, nil
}

type RedeemCouponCodeRequest struct {
	Code string `json:"Code"`
}

type RedeemCouponCodeResponse struct {
	Data struct {
		Transaction struct {
			AmountUSD string `json:"amount_usd"`
		} `json:"transaction"`
	} `json:"data"`
}

func (s AuthHTTPStore) RedeemCouponCode(organizationID string, code string) (*RedeemCouponCodeResponse, error) {
	var result RedeemCouponCodeResponse
	path := orgPath + "/" + organizationID + "/credits/code/redeem"
	req := RedeemCouponCodeRequest{
		Code: code,
	}

	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetResult(&result).
		SetBody(req).
		Post(path)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}
