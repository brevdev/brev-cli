package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplaneapiv1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type instanceCatalogTestHandler struct {
	devplaneapiv1connect.UnimplementedInstanceServiceHandler
	gotAuth                   string
	gotOrgID                  string
	gotConnectProtocolVersion string
	gotIncludeCPU             bool
	gotSkipAccessFilter       bool
}

func (h *instanceCatalogTestHandler) ListPublicInstanceType(
	_ context.Context,
	req *connect.Request[devplaneapiv1.ListPublicInstanceTypeRequest],
) (*connect.Response[devplaneapiv1.ListPublicInstanceTypeResponse], error) {
	h.gotAuth = req.Header().Get("Authorization")
	h.gotConnectProtocolVersion = req.Header().Get("Connect-Protocol-Version")
	h.gotIncludeCPU = req.Msg.GetOptions().GetIncludeCpu()
	return connect.NewResponse(&devplaneapiv1.ListPublicInstanceTypeResponse{
		Items: []*devplaneapiv1.InstanceType{{
			Type:                   "h100-1x",
			SupportedArchitectures: []string{"amd64"},
			Memory:                 "128 GB",
			MemoryBytes:            &devplaneapiv1.Bytes{Value: 128, Unit: "GB"},
			Vcpu:                   16,
			SupportedGpus: []*devplaneapiv1.Gpu{{
				Count:        1,
				Name:         "H100",
				Manufacturer: "NVIDIA",
				Memory:       "80 GB",
				MemoryBytes:  &devplaneapiv1.Bytes{Value: 80, Unit: "GB"},
			}},
			SupportedStorage: []*devplaneapiv1.Storage{{
				Count:     1,
				Size:      "100 GB",
				Type:      "SSD",
				MinSize:   "50 GB",
				MaxSize:   "2 TB",
				SizeBytes: &devplaneapiv1.Bytes{Value: 100, Unit: "GB"},
				PricePerGbHr: &devplaneapiv1.CurrencyAmount{
					Currency: "USD",
					Amount:   "0.0001",
				},
			}},
			BasePrice: &devplaneapiv1.CurrencyAmount{
				Currency: "USD",
				Amount:   "2.50",
			},
			Location:               "us-east-1",
			SubLocation:            "use1-az1",
			AvailableLocations:     []string{"us-east-1", "us-west-2"},
			Provider:               "aws",
			CloudCredId:            "cc-public-1",
			EstimatedDeployTime:    "5m",
			Stoppable:              true,
			Rebootable:             true,
			CanModifyFirewallRules: true,
		}},
	}), nil
}

func (h *instanceCatalogTestHandler) ListOrganizationAvailableInstanceTypes(
	_ context.Context,
	req *connect.Request[devplaneapiv1.ListOrganizationAvailableInstanceTypesRequest],
) (*connect.Response[devplaneapiv1.ListOrganizationAvailableInstanceTypesResponse], error) {
	h.gotAuth = req.Header().Get("Authorization")
	h.gotOrgID = req.Msg.GetOrganizationId()
	h.gotConnectProtocolVersion = req.Header().Get("Connect-Protocol-Version")
	h.gotSkipAccessFilter = req.Msg.GetOptions().GetSkipAccessFilter()
	items := []*devplaneapiv1.InstanceType{}
	if h.gotSkipAccessFilter {
		items = append(items, &devplaneapiv1.InstanceType{
			Type:        "verda_RTXPro6000",
			CloudCredId: "cc-org-1",
			CloudCred: &devplaneapiv1.CloudCredMetadata{
				CloudCredId: "cc-org-1",
				ProviderId:  "shadeform",
				Name:        "Shadeform",
				TenantType:  devplaneapiv1.TenantType_TENANT_TYPE_ISOLATED,
			},
			AvailableLocations: []string{"us-central-1"},
			IsAvailable:        true,
		})
	}
	return connect.NewResponse(&devplaneapiv1.ListOrganizationAvailableInstanceTypesResponse{
		Items: items,
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
	assert.True(t, catalogHandler.gotSkipAccessFilter)
	if assert.Len(t, resp.AllInstanceTypes, 1) {
		assert.Equal(t, "verda_RTXPro6000", resp.AllInstanceTypes[0].Type)
		assert.Equal(t, "cc-org-1", resp.GetCloudCredID("verda_RTXPro6000"))
	}
}

func TestGetInstanceTypesUsesDevPlanePublicRPC(t *testing.T) {
	catalogHandler := &instanceCatalogTestHandler{}
	_, connectHandler := devplaneapiv1connect.NewInstanceServiceHandler(catalogHandler)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/devplaneapi.v1.InstanceService/ListPublicInstanceType", r.URL.Path)
		connectHandler.ServeHTTP(w, r)
	}))
	defer server.Close()

	t.Setenv("BREV_PUBLIC_API_URL", server.URL)

	resp, err := (NoAuthHTTPStore{}).GetInstanceTypes(true)
	require.NoError(t, err)

	assert.Empty(t, catalogHandler.gotAuth)
	assert.Equal(t, "1", catalogHandler.gotConnectProtocolVersion)
	assert.True(t, catalogHandler.gotIncludeCPU)
	if assert.Len(t, resp.Items, 1) {
		item := resp.Items[0]
		assert.Equal(t, "h100-1x", item.Type)
		assert.Equal(t, []string{"amd64"}, item.SupportedArchitectures)
		assert.Equal(t, "128 GB", item.Memory)
		assert.Equal(t, int64(128), item.InstanceMemoryBytes.Value)
		assert.Equal(t, "GB", item.InstanceMemoryBytes.Unit)
		assert.Equal(t, 16, item.VCPU)
		assert.Equal(t, gpusearch.BasePrice{Currency: "USD", Amount: "2.50"}, item.BasePrice)
		assert.Equal(t, "us-east-1", item.Location)
		assert.Equal(t, "use1-az1", item.SubLocation)
		assert.Equal(t, []string{"us-east-1", "us-west-2"}, item.AvailableLocations)
		assert.Equal(t, "aws", item.Provider)
		assert.Equal(t, "cc-public-1", item.CloudCredID)
		assert.Equal(t, "5m", item.EstimatedDeployTime)
		assert.True(t, item.Stoppable)
		assert.True(t, item.Rebootable)
		assert.True(t, item.CanModifyFirewallRules)
		if assert.Len(t, item.SupportedGPUs, 1) {
			assert.Equal(t, gpusearch.MemoryBytes{Value: 80, Unit: "GB"}, item.SupportedGPUs[0].MemoryBytes)
		}
		if assert.Len(t, item.SupportedStorage, 1) {
			storage := item.SupportedStorage[0]
			assert.Equal(t, "100 GB", storage.Size)
			assert.Equal(t, "50 GB", storage.MinSize)
			assert.Equal(t, "2 TB", storage.MaxSize)
			assert.Equal(t, gpusearch.MemoryBytes{Value: 100, Unit: "GB"}, storage.SizeBytes)
			assert.Equal(t, gpusearch.BasePrice{Currency: "USD", Amount: "0.0001"}, storage.PricePerGBHr)
		}
	}
}
