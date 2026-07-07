package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type devPlaneUserService interface {
	GetCurrentUser(context.Context) (*nodev1.User, string, error)
	GetCurrentUserKeys(context.Context) (*entity.UserKeys, error)
	GetUser(context.Context, string) (*nodev1.User, error)
	SetUserBlocked(context.Context, string, bool) (*nodev1.User, error)
}

type devPlaneOrganizationService interface {
	ListOrganizations(context.Context, []string) ([]*nodev1.Organization, error)
	GetOrganization(context.Context, string) (*nodev1.Organization, error)
}

type devPlaneServices struct {
	user         devPlaneUserService
	organization devPlaneOrganizationService
}

type authHTTPStoreTransport struct {
	store *AuthHTTPStore
	base  http.RoundTripper
}

func (t *authHTTPStoreTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.store.GetAccessToken()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return resp, nil
}

func (s *AuthHTTPStore) devPlaneHTTPClient() *http.Client {
	return &http.Client{Transport: &authHTTPStoreTransport{store: s, base: http.DefaultTransport}}
}

func (s *AuthHTTPStore) withDevPlaneServices(services *devPlaneServices) *AuthHTTPStore {
	copy := *s
	copy.devPlane = services
	return &copy
}

func (s *AuthHTTPStore) devPlaneServiceClients() *devPlaneServices {
	if s.devPlane != nil {
		return s.devPlane
	}
	httpClient := s.devPlaneHTTPClient()
	baseURL := config.GlobalConfig.GetBrevPublicAPIURL()
	return &devPlaneServices{
		user: &generatedDevPlaneUserService{client: nodev1connect.NewUserServiceClient(httpClient, baseURL)},
		organization: &generatedDevPlaneOrganizationService{
			client: nodev1connect.NewOrganizationServiceClient(httpClient, baseURL),
		},
	}
}

type generatedDevPlaneUserService struct {
	client nodev1connect.UserServiceClient
}

func (s *generatedDevPlaneUserService) GetCurrentUser(ctx context.Context) (*nodev1.User, string, error) {
	response, err := s.client.GetCurrentUser(ctx, connect.NewRequest(&nodev1.GetCurrentUserRequest{}))
	if err != nil {
		return nil, "", breverrors.WrapAndTrace(err)
	}
	// The public SSH-key response field is added by Set 1. Keep the local
	// contract ready and populate it after the landed Buf revision is available.
	return response.Msg.GetUser(), "", nil
}

func (s *generatedDevPlaneUserService) GetCurrentUserKeys(context.Context) (*entity.UserKeys, error) {
	return nil, fmt.Errorf("GetCurrentUserSSHKeys requires the published Set 1 Buf revision")
}

func (s *generatedDevPlaneUserService) GetUser(ctx context.Context, userID string) (*nodev1.User, error) {
	response, err := s.client.GetUser(ctx, connect.NewRequest(&nodev1.GetUserRequest{
		UserId: userID,
		AttachedDataOptions: &nodev1.UserAttachedDataOptions{
			Emails:               true,
			ExternalIdentities:   true,
			OrganizationAccesses: true,
		},
	}))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return response.Msg.GetUser(), nil
}

func (s *generatedDevPlaneUserService) SetUserBlocked(context.Context, string, bool) (*nodev1.User, error) {
	return nil, fmt.Errorf("SetUserBlocked requires the published Set 1 Buf revision")
}

type generatedDevPlaneOrganizationService struct {
	client nodev1connect.OrganizationServiceClient
}

func (s *generatedDevPlaneOrganizationService) ListOrganizations(ctx context.Context, ids []string) ([]*nodev1.Organization, error) {
	organizations := []*nodev1.Organization{}
	pageToken := ""
	for {
		response, err := s.client.ListOrganization(ctx, connect.NewRequest(&nodev1.ListOrganizationRequest{
			PageParams: &nodev1.PageParams{PageSize: 1000, PageToken: pageToken},
			Options:    &nodev1.ListOrganizationOptions{Ids: ids},
		}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		organizations = append(organizations, response.Msg.GetItems()...)
		pageToken = response.Msg.GetNextPageToken()
		if pageToken == "" {
			return organizations, nil
		}
	}
}

func (s *generatedDevPlaneOrganizationService) GetOrganization(ctx context.Context, id string) (*nodev1.Organization, error) {
	response, err := s.client.GetOrganization(ctx, connect.NewRequest(&nodev1.GetOrganizationRequest{OrganizationId: id}))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return response.Msg.GetOrganization(), nil
}

func mapDevPlaneUser(user *nodev1.User, publicSSHKey string) (*entity.User, error) {
	if user == nil {
		return nil, fmt.Errorf("dev-plane returned no user")
	}
	metadata := struct {
		BaseWorkspaceRepo string                 `json:"baseWorkspaceRepo"`
		UserSetupExecPath string                 `json:"userSetupExecPath"`
		WorkspacePassword string                 `json:"workspacePassword"`
		GlobalUserType    entity.GlobalUserType  `json:"globalUserType"`
		IDEConfig         entity.IDEConfig       `json:"ideConfig"`
		OnboardingData    map[string]interface{} `json:"onboardingData"`
	}{}
	if user.GetMetadata() != nil {
		data, err := json.Marshal(user.GetMetadata().AsMap())
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if err := json.Unmarshal(data, &metadata); err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}
	var externalIdentities []*entity.ExternalIdentity
	for _, identity := range user.GetExternalIdentities() {
		externalIdentities = append(externalIdentities, &entity.ExternalIdentity{
			IdentityID: identity.GetIdentityId(),
			Provider:   identity.GetProvider(),
			ExternalID: identity.GetExternalId(),
		})
	}
	return &entity.User{
		ID:                 user.GetUserId(),
		PublicKey:          publicSSHKey,
		Username:           user.GetUsername(),
		Name:               user.GetDisplayName(),
		Email:              user.GetDefaultEmail(),
		ExternalIdentities: externalIdentities,
		WorkspacePassword:  metadata.WorkspacePassword,
		BaseWorkspaceRepo:  metadata.BaseWorkspaceRepo,
		GlobalUserType:     metadata.GlobalUserType,
		IdeConfig:          metadata.IDEConfig,
		OnboardingData:     metadata.OnboardingData,
	}, nil
}

func mapDevPlaneOrganization(organization *nodev1.Organization) (*entity.Organization, error) {
	if organization == nil {
		return nil, fmt.Errorf("dev-plane returned no organization")
	}
	return &entity.Organization{
		ID:   organization.GetOrganizationId(),
		Name: organization.GetDisplayName(),
	}, nil
}
