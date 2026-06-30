package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	devplaneapiv1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplaneapiv1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	authpkg "github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetActiveOrganization(t *testing.T) {
	fs := MakeMockAuthHTTPStore()
	org, err := fs.GetActiveOrganizationOrNil()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Nil(t, org) {
		return
	}
}

func TestGetOrganizations(t *testing.T) {
	fs := newStoreWithOrganizationService(t, &fakeOrganizationService{
		listOrganizationFunc: func(ctx context.Context, req *connect.Request[devplaneapiv1.ListOrganizationRequest]) (*connect.Response[devplaneapiv1.ListOrganizationResponse], error) {
			assert.Equal(t, "Bearer mock-token", req.Header().Get("Authorization"))
			return connect.NewResponse(&devplaneapiv1.ListOrganizationResponse{
				Items: []*devplaneapiv1.Organization{{
					OrganizationId: "1",
					DisplayName:    "test",
				}},
			}), nil
		},
	})

	org, err := fs.GetOrganizations(nil)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, org) {
		return
	}

	if !assert.Equal(t, []entity.Organization{{ID: "1", Name: "test"}}, org) {
		return
	}
}

func TestGetActiveOrganization_APIKeyUsesCredentialOrg(t *testing.T) {
	apiKey := authpkg.BrevAPIKeyPrefix + "test-key"
	fileStore, _, _ := newAuthTokenTestStore(t)
	s := fileStore.WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{token: &apiKey}, "https://api.test"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected legacy call", http.StatusTeapot)
	}))
	defer server.Close()
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)

	require.NoError(t, s.SaveAuthTokens(entity.AuthTokens{
		APIKey:      apiKey,
		APIKeyOrgID: "org-api-key",
	}))
	require.NoError(t, s.SetDefaultOrganization(&entity.Organization{
		ID:   "org-cached",
		Name: "cached-org",
	}))

	org, err := s.GetActiveOrganizationOrDefault()

	require.NoError(t, err)
	require.NotNil(t, org)
	assert.Equal(t, "org-api-key", org.ID)
	assert.Equal(t, "org-api-key", org.Name)
}

func TestGetActiveOrganization_APIKeyUsesCredentialOrgNameWhenAvailable(t *testing.T) {
	apiKey := authpkg.BrevAPIKeyPrefix + "test-key"
	fileStore, _, _ := newAuthTokenTestStore(t)
	s := fileStore.WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{token: &apiKey}, "https://api.test"))

	require.NoError(t, s.SaveAuthTokens(entity.AuthTokens{
		APIKey:      apiKey,
		APIKeyOrgID: "org-api-key",
	}))
	expected := &entity.Organization{
		ID:   "org-api-key",
		Name: "friendly-org",
	}
	withOrganizationService(t, &fakeOrganizationService{
		getOrganizationFunc: func(ctx context.Context, req *connect.Request[devplaneapiv1.GetOrganizationRequest]) (*connect.Response[devplaneapiv1.GetOrganizationResponse], error) {
			assert.Equal(t, "org-api-key", req.Msg.GetOrganizationId())
			assert.Equal(t, "Bearer "+apiKey, req.Header().Get("Authorization"))
			return connect.NewResponse(&devplaneapiv1.GetOrganizationResponse{
				Organization: &devplaneapiv1.Organization{
					OrganizationId: "org-api-key",
					DisplayName:    "friendly-org",
				},
			}), nil
		},
	})

	org, err := s.GetActiveOrganizationOrDefault()

	require.NoError(t, err)
	require.NotNil(t, org)
	assert.Equal(t, expected.ID, org.ID)
	assert.Equal(t, expected.Name, org.Name)
}

func TestGetOrganizationsFiltersNameCaseInsensitive(t *testing.T) {
	fs := newStoreWithOrganizationService(t, &fakeOrganizationService{
		listOrganizationFunc: func(ctx context.Context, req *connect.Request[devplaneapiv1.ListOrganizationRequest]) (*connect.Response[devplaneapiv1.ListOrganizationResponse], error) {
			return connect.NewResponse(&devplaneapiv1.ListOrganizationResponse{
				Items: []*devplaneapiv1.Organization{
					{OrganizationId: "1", DisplayName: "TEST"},
					{OrganizationId: "2", DisplayName: "Other"},
				},
			}), nil
		},
	})

	org, err := fs.GetOrganizations(&GetOrganizationsOptions{Name: "test"})
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, org) {
		return
	}

	if !assert.Equal(t, []entity.Organization{{ID: "1", Name: "TEST"}}, org) {
		return
	}
}

func TestCreateOrganization(t *testing.T) {
	fs := newStoreWithOrganizationService(t, &fakeOrganizationService{
		createOrganizationFunc: func(ctx context.Context, req *connect.Request[devplaneapiv1.CreateOrganizationRequest]) (*connect.Response[devplaneapiv1.CreateOrganizationResponse], error) {
			assert.Equal(t, "test", req.Msg.GetDisplayName())
			assert.Equal(t, "Bearer mock-token", req.Header().Get("Authorization"))
			return connect.NewResponse(&devplaneapiv1.CreateOrganizationResponse{
				Organization: &devplaneapiv1.Organization{
					OrganizationId: "1",
					DisplayName:    "test",
				},
			}), nil
		},
	})

	org, err := fs.CreateOrganization(CreateOrganizationRequest{Name: "test"})
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, org) {
		return
	}

	if !assert.Equal(t, &entity.Organization{ID: "1", Name: "test"}, org) {
		return
	}
}

func TestGetOrganizationUsesDevPlaneRPC(t *testing.T) {
	fs := newStoreWithOrganizationService(t, &fakeOrganizationService{
		getOrganizationFunc: func(ctx context.Context, req *connect.Request[devplaneapiv1.GetOrganizationRequest]) (*connect.Response[devplaneapiv1.GetOrganizationResponse], error) {
			assert.Equal(t, "org-1", req.Msg.GetOrganizationId())
			assert.Equal(t, "Bearer mock-token", req.Header().Get("Authorization"))
			return connect.NewResponse(&devplaneapiv1.GetOrganizationResponse{
				Organization: &devplaneapiv1.Organization{
					OrganizationId: "org-1",
					DisplayName:    "Test Org",
				},
			}), nil
		},
	})

	org, err := fs.GetOrganization("org-1")

	require.NoError(t, err)
	assert.Equal(t, &entity.Organization{ID: "org-1", Name: "Test Org"}, org)
}

func newStoreWithOrganizationService(t *testing.T, svc devplaneapiv1connect.OrganizationServiceHandler) *AuthHTTPStore {
	t.Helper()
	token := "mock-token"
	legacyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected legacy REST call", http.StatusTeapot)
	}))
	t.Cleanup(legacyServer.Close)
	fs := MakeMockAuthHTTPStore()
	fs.authHTTPClient = NewAuthHTTPClient(MockAuth{token: &token}, legacyServer.URL)
	withOrganizationService(t, svc)
	return fs
}

func withOrganizationService(t *testing.T, svc devplaneapiv1connect.OrganizationServiceHandler) {
	t.Helper()
	_, handler := devplaneapiv1connect.NewOrganizationServiceHandler(svc)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)
}

type fakeOrganizationService struct {
	listOrganizationFunc   func(context.Context, *connect.Request[devplaneapiv1.ListOrganizationRequest]) (*connect.Response[devplaneapiv1.ListOrganizationResponse], error)
	getOrganizationFunc    func(context.Context, *connect.Request[devplaneapiv1.GetOrganizationRequest]) (*connect.Response[devplaneapiv1.GetOrganizationResponse], error)
	createOrganizationFunc func(context.Context, *connect.Request[devplaneapiv1.CreateOrganizationRequest]) (*connect.Response[devplaneapiv1.CreateOrganizationResponse], error)
}

func (f *fakeOrganizationService) ListOrganization(ctx context.Context, req *connect.Request[devplaneapiv1.ListOrganizationRequest]) (*connect.Response[devplaneapiv1.ListOrganizationResponse], error) {
	if f.listOrganizationFunc != nil {
		return f.listOrganizationFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, assert.AnError)
}

func (f *fakeOrganizationService) GetOrganization(ctx context.Context, req *connect.Request[devplaneapiv1.GetOrganizationRequest]) (*connect.Response[devplaneapiv1.GetOrganizationResponse], error) {
	if f.getOrganizationFunc != nil {
		return f.getOrganizationFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, assert.AnError)
}

func (f *fakeOrganizationService) CreateOrganization(ctx context.Context, req *connect.Request[devplaneapiv1.CreateOrganizationRequest]) (*connect.Response[devplaneapiv1.CreateOrganizationResponse], error) {
	if f.createOrganizationFunc != nil {
		return f.createOrganizationFunc(ctx, req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, assert.AnError)
}

func (f *fakeOrganizationService) CreateOrganizationForUser(context.Context, *connect.Request[devplaneapiv1.CreateOrganizationForUserRequest]) (*connect.Response[devplaneapiv1.CreateOrganizationForUserResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, assert.AnError)
}

func (f *fakeOrganizationService) DeleteOrganization(context.Context, *connect.Request[devplaneapiv1.DeleteOrganizationRequest]) (*connect.Response[devplaneapiv1.DeleteOrganizationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, assert.AnError)
}

func (f *fakeOrganizationService) UpdateOrganization(context.Context, *connect.Request[devplaneapiv1.UpdateOrganizationRequest]) (*connect.Response[devplaneapiv1.UpdateOrganizationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, assert.AnError)
}

func (f *fakeOrganizationService) AddCreditsToOrganization(context.Context, *connect.Request[devplaneapiv1.AddCreditsToOrganizationRequest]) (*connect.Response[devplaneapiv1.AddCreditsToOrganizationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, assert.AnError)
}

func (f *fakeOrganizationService) RemoveCreditsFromOrganization(context.Context, *connect.Request[devplaneapiv1.RemoveCreditsFromOrganizationRequest]) (*connect.Response[devplaneapiv1.RemoveCreditsFromOrganizationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, assert.AnError)
}

func (f *fakeOrganizationService) ListOrganizationMembers(context.Context, *connect.Request[devplaneapiv1.ListOrganizationMembersRequest]) (*connect.Response[devplaneapiv1.ListOrganizationMembersResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, assert.AnError)
}
