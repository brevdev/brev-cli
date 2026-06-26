package store

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	authpkg "github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/jarcoal/httpmock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOrganizationService struct {
	nodev1connect.UnimplementedOrganizationServiceHandler
	listMembersFn func(http.Header, *nodev1.ListOrganizationMembersRequest) (*nodev1.ListOrganizationMembersResponse, error)
}

func (f *fakeOrganizationService) ListOrganizationMembers(_ context.Context, req *connect.Request[nodev1.ListOrganizationMembersRequest]) (*connect.Response[nodev1.ListOrganizationMembersResponse], error) {
	resp, err := f.listMembersFn(req.Header(), req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

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
	fs := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(fs.authHTTPClient.restyClient.GetClient())
	defer httpmock.DeactivateAndReset()

	expected := []entity.Organization{{
		ID:   "1",
		Name: "test",
	}}
	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", fs.authHTTPClient.restyClient.BaseURL, orgPath)
	httpmock.RegisterResponder("GET", url, res)

	org, err := fs.GetOrganizations(nil)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, org) {
		return
	}

	if !assert.Equal(t, expected, org) {
		return
	}
}

func TestListOrganizationMembersUsesDevPlaneRPC(t *testing.T) {
	var gotAuth string
	var gotOrgID string
	svc := &fakeOrganizationService{
		listMembersFn: func(header http.Header, msg *nodev1.ListOrganizationMembersRequest) (*nodev1.ListOrganizationMembersResponse, error) {
			gotAuth = header.Get("Authorization")
			gotOrgID = msg.GetOrganizationId()
			return &nodev1.ListOrganizationMembersResponse{
				Items: []*nodev1.OrganizationMember{
					{UserId: "user_1", DisplayName: "Alice", DefaultEmail: "alice@example.com"},
					{UserId: "user_2", DisplayName: "Bob", DefaultEmail: "bob@example.com"},
				},
			}, nil
		},
	}
	_, handler := nodev1connect.NewOrganizationServiceHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	token := "tok"
	fileStore, _, _ := newAuthTokenTestStore(t)
	s := fileStore.WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{token: &token}, server.URL))

	members, err := s.ListOrganizationMembers(context.Background(), "org_123")

	require.NoError(t, err)
	require.Len(t, members, 2)
	assert.Equal(t, "Bearer tok", gotAuth)
	assert.Equal(t, "org_123", gotOrgID)
	assert.Equal(t, "user_1", members[0].GetUserId())
	assert.True(t, strings.Contains(members[1].GetDefaultEmail(), "@"))
}

func TestGetActiveOrganization_APIKeyUsesCredentialOrg(t *testing.T) {
	apiKey := authpkg.BrevAPIKeyPrefix + "test-key"
	fileStore, _, _ := newAuthTokenTestStore(t)
	s := fileStore.WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{token: &apiKey}, "https://api.test"))
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())
	defer httpmock.DeactivateAndReset()

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
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())
	defer httpmock.DeactivateAndReset()

	require.NoError(t, s.SaveAuthTokens(entity.AuthTokens{
		APIKey:      apiKey,
		APIKeyOrgID: "org-api-key",
	}))
	expected := &entity.Organization{
		ID:   "org-api-key",
		Name: "friendly-org",
	}
	res, err := httpmock.NewJsonResponder(200, expected)
	require.NoError(t, err)
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(orgIDPathPattern, "org-api-key"))
	httpmock.RegisterResponder("GET", url, res)

	org, err := s.GetActiveOrganizationOrDefault()

	require.NoError(t, err)
	require.NotNil(t, org)
	assert.Equal(t, expected.ID, org.ID)
	assert.Equal(t, expected.Name, org.Name)
}

func TestGetOrganizationsFiltersNameCaseInsensitive(t *testing.T) {
	fs := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(fs.authHTTPClient.restyClient.GetClient())
	defer httpmock.DeactivateAndReset()

	expected := []entity.Organization{{
		ID:   "1",
		Name: "TEST",
	}}
	orgs := []entity.Organization{
		expected[0],
		{
			ID:   "2",
			Name: "Other",
		},
	}
	res, err := httpmock.NewJsonResponder(200, orgs)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", fs.authHTTPClient.restyClient.BaseURL, orgPath)
	httpmock.RegisterResponder("GET", url, res)

	org, err := fs.GetOrganizations(&GetOrganizationsOptions{Name: "test"})
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, org) {
		return
	}

	if !assert.Equal(t, expected, org) {
		return
	}
}

func TestCreateOrganization(t *testing.T) {
	fs := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(fs.authHTTPClient.restyClient.GetClient())
	defer httpmock.DeactivateAndReset()

	expected := &entity.Organization{
		ID:   "1",
		Name: "test",
	}
	res, err := httpmock.NewJsonResponder(201, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", fs.authHTTPClient.restyClient.BaseURL, orgPath)
	httpmock.RegisterResponder("POST", url, res)

	org, err := fs.CreateOrganization(CreateOrganizationRequest{Name: expected.Name})
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, org) {
		return
	}

	if !assert.Equal(t, expected, org) {
		return
	}
}
