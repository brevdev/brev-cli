package store

import (
	"encoding/json"

	devplaneapiv1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	instanceTypesAPIPath              = "v1/instance/types"
	organizationAvailableTypesRPCPath = "devplaneapi.v1.InstanceService/ListOrganizationAvailableInstanceTypes"
	tenantTypeShared                  = "shared"
	tenantTypeIsolated                = "isolated"
)

// GetInstanceTypes fetches all available instance types from the public API
func (s NoAuthHTTPStore) GetInstanceTypes(includeCPU bool) (*gpusearch.InstanceTypesResponse, error) {
	return fetchInstanceTypes(includeCPU)
}

// GetInstanceTypes fetches all available instance types from the public API
func (s AuthHTTPStore) GetInstanceTypes(includeCPU bool) (*gpusearch.InstanceTypesResponse, error) {
	return fetchInstanceTypes(includeCPU)
}

// fetchInstanceTypes fetches instance types from the public Brev API
func fetchInstanceTypes(includeCPU bool) (*gpusearch.InstanceTypesResponse, error) {
	cfg := config.NewConstants()
	client := NewRestyClient(cfg.GetBrevPublicAPIURL())

	req := client.R().
		SetHeader("Accept", "application/json")
	if includeCPU {
		req.SetQueryParam("include_cpu", "true")
	}
	res, err := req.Get(instanceTypesAPIPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	var result gpusearch.InstanceTypesResponse
	err = json.Unmarshal(res.Body(), &result)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &result, nil
}

// GetAllInstanceTypesWithCloudCreds fetches org-scoped instance types from dev-plane's public Connect API.
func (s AuthHTTPStore) GetAllInstanceTypesWithCloudCreds(orgID string) (*gpusearch.AllInstanceTypesResponse, error) {
	publicClient := NewAuthHTTPClient(s.authHTTPClient.auth, config.NewConstants().GetBrevPublicAPIURL()).restyClient

	res, err := publicClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"organizationId": orgID,
			"options": map[string]interface{}{
				"includeUnavailable": false,
				"includePreemptible": false,
				"includeCpu":         true,
				"uniqueInstanceType": true,
				"skipAccessFilter":   false,
			},
		}).
		Post(organizationAvailableTypesRPCPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	var protoResp devplaneapiv1.ListInstanceTypeResponse
	if err := protojson.Unmarshal(res.Body(), &protoResp); err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return mapProtoInstanceTypesToGPUSearchResponse(protoResp.Items), nil
}

// GetAllInstanceTypesWithWorkspaceGroups is a compatibility alias while create still accepts workspaceGroupId.
func (s AuthHTTPStore) GetAllInstanceTypesWithWorkspaceGroups(orgID string) (*gpusearch.AllInstanceTypesResponse, error) {
	return s.GetAllInstanceTypesWithCloudCreds(orgID)
}

func mapProtoInstanceTypesToGPUSearchResponse(instanceTypes []*devplaneapiv1.InstanceType) *gpusearch.AllInstanceTypesResponse {
	items := make([]gpusearch.InstanceType, 0, len(instanceTypes))
	for _, instanceType := range instanceTypes {
		if instanceType == nil {
			continue
		}
		item := gpusearch.InstanceType{
			Type:                   instanceType.GetType(),
			VCPU:                   int(instanceType.GetVcpu()),
			Location:               instanceType.GetLocation(),
			SubLocation:            instanceType.GetSubLocation(),
			AvailableLocations:     instanceType.GetAvailableLocations(),
			Provider:               instanceType.GetProvider(),
			CloudCredID:            instanceType.GetCloudCredId(),
			EstimatedDeployTime:    instanceType.GetEstimatedDeployTime(),
			Stoppable:              instanceType.GetStoppable(),
			Rebootable:             instanceType.GetRebootable(),
			CanModifyFirewallRules: instanceType.GetCanModifyFirewallRules(),
		}
		if basePrice := instanceType.GetBasePrice(); basePrice != nil {
			item.BasePrice = gpusearch.BasePrice{
				Currency: basePrice.GetCurrency(),
				Amount:   basePrice.GetAmount(),
			}
		}
		for _, gpu := range instanceType.GetSupportedGpus() {
			item.SupportedGPUs = append(item.SupportedGPUs, gpusearch.GPU{
				Count:        int(gpu.GetCount()),
				Name:         gpu.GetName(),
				Manufacturer: gpu.GetManufacturer(),
				Memory:       gpu.GetMemory(),
			})
		}
		if cloudCred := instanceType.GetCloudCred(); cloudCred != nil {
			mappedCloudCred := gpusearch.CloudCred{
				ID:           cloudCred.GetCloudCredId(),
				Name:         cloudCred.GetName(),
				PlatformType: cloudCred.GetProviderId(),
				TenantType:   mapTenantType(cloudCred.GetTenantType()),
			}
			item.CloudCreds = []gpusearch.CloudCred{mappedCloudCred}
			item.WorkspaceGroups = []gpusearch.WorkspaceGroup{{
				ID:           mappedCloudCred.ID,
				Name:         mappedCloudCred.Name,
				PlatformType: mappedCloudCred.PlatformType,
			}}
		}
		items = append(items, item)
	}
	return &gpusearch.AllInstanceTypesResponse{AllInstanceTypes: items}
}

func mapTenantType(tenantType devplaneapiv1.TenantType) string {
	switch tenantType {
	case devplaneapiv1.TenantType_TENANT_TYPE_ISOLATED:
		return tenantTypeIsolated
	case devplaneapiv1.TenantType_TENANT_TYPE_SHARED:
		return tenantTypeShared
	default:
		return ""
	}
}
