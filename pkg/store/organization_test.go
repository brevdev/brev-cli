package store

import (
	"fmt"
	"testing"

	authpkg "github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/jarcoal/httpmock"

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
