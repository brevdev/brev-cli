package store

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllInstanceTypesWithCloudCredsUsesDevPlanePublicAPI(t *testing.T) {
	var gotAuth string
	var gotOrgID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/devplaneapi.v1.InstanceService/ListOrganizationAvailableInstanceTypes", r.URL.Path)
		gotAuth = r.Header.Get("Authorization")

		var req struct {
			OrganizationID string `json:"organizationId"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		gotOrgID = req.OrganizationID

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [{
				"type": "h100-1x",
				"cloudCredId": "cc-org-1",
				"cloudCred": {
					"cloudCredId": "cc-org-1",
					"providerId": "aws",
					"name": "Org AWS",
					"tenantType": "TENANT_TYPE_ISOLATED"
				},
				"availableLocations": ["us-east-1"],
				"isAvailable": true
			}]
		}`))
	}))
	defer server.Close()

	t.Setenv("BREV_PUBLIC_API_URL", server.URL)
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("legacy brev-deploy API should not be called: %s", r.URL.Path)
	}))
	defer legacy.Close()

	token := "tok"
	s := MakeMockNoHTTPStore().WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{token: &token}, legacy.URL))

	resp, err := s.GetAllInstanceTypesWithCloudCreds("org-1")
	require.NoError(t, err)

	assert.Equal(t, "Bearer tok", gotAuth)
	assert.Equal(t, "org-1", gotOrgID)
	if assert.Len(t, resp.AllInstanceTypes, 1) {
		assert.Equal(t, "h100-1x", resp.AllInstanceTypes[0].Type)
		assert.Equal(t, "cc-org-1", resp.GetCloudCredID("h100-1x"))
		assert.Equal(t, "cc-org-1", resp.GetWorkspaceGroupID("h100-1x"))
	}
}
