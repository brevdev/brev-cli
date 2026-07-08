package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplaneapiv1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type instanceCatalogTestHandler struct {
	devplaneapiv1connect.UnimplementedInstanceServiceHandler
	gotAuth                   string
	gotOrgID                  string
	gotConnectProtocolVersion string
}

func (h *instanceCatalogTestHandler) ListOrganizationAvailableInstanceTypes(
	_ context.Context,
	req *connect.Request[devplaneapiv1.ListOrganizationAvailableInstanceTypesRequest],
) (*connect.Response[devplaneapiv1.ListOrganizationAvailableInstanceTypesResponse], error) {
	h.gotAuth = req.Header().Get("Authorization")
	h.gotOrgID = req.Msg.GetOrganizationId()
	h.gotConnectProtocolVersion = req.Header().Get("Connect-Protocol-Version")
	return connect.NewResponse(&devplaneapiv1.ListOrganizationAvailableInstanceTypesResponse{
		Items: []*devplaneapiv1.InstanceType{{
			Type:        "h100-1x",
			CloudCredId: "cc-org-1",
			CloudCred: &devplaneapiv1.CloudCredMetadata{
				CloudCredId: "cc-org-1",
				ProviderId:  "aws",
				Name:        "Org AWS",
				TenantType:  devplaneapiv1.TenantType_TENANT_TYPE_ISOLATED,
			},
			AvailableLocations: []string{"us-east-1"},
			IsAvailable:        true,
		}},
	}), nil
}

func TestGetAllInstanceTypesWithCloudCredsUsesDevPlanePublicAPI(t *testing.T) {
	catalogHandler := &instanceCatalogTestHandler{}
	_, connectHandler := devplaneapiv1connect.NewInstanceServiceHandler(catalogHandler)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/devplaneapi.v1.InstanceService/ListOrganizationAvailableInstanceTypes", r.URL.Path)
		connectHandler.ServeHTTP(w, r)
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

	assert.Equal(t, "Bearer tok", catalogHandler.gotAuth)
	assert.Equal(t, "1", catalogHandler.gotConnectProtocolVersion)
	assert.Equal(t, "org-1", catalogHandler.gotOrgID)
	if assert.Len(t, resp.AllInstanceTypes, 1) {
		assert.Equal(t, "h100-1x", resp.AllInstanceTypes[0].Type)
		assert.Equal(t, "cc-org-1", resp.GetCloudCredID("h100-1x"))
		assert.Equal(t, "cc-org-1", resp.GetWorkspaceGroupID("h100-1x"))
	}
}
