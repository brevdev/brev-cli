package store

import (
	"context"
	"net/http"

	"buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplaneapiv1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const (
	tenantTypeShared   = "shared"
	tenantTypeIsolated = "isolated"
)

// GetInstanceTypes fetches all available instance types from the public API
func (s NoAuthHTTPStore) GetInstanceTypes(includeCPU bool) (*gpusearch.InstanceTypesResponse, error) {
	return fetchInstanceTypes(includeCPU)
}

// GetInstanceTypes fetches all available instance types from the public API
func (s AuthHTTPStore) GetInstanceTypes(includeCPU bool) (*gpusearch.InstanceTypesResponse, error) {
	return fetchInstanceTypes(includeCPU)
}

// fetchInstanceTypes fetches instance types from dev-plane's public Connect API.
func fetchInstanceTypes(includeCPU bool) (*gpusearch.InstanceTypesResponse, error) {
	client := devplaneapiv1connect.NewInstanceServiceClient(
		http.DefaultClient,
		config.NewConstants().GetBrevPublicAPIURL(),
	)
	skipAccessFilter := false
	res, err := client.ListPublicInstanceType(context.Background(), connect.NewRequest(&devplaneapiv1.ListPublicInstanceTypeRequest{
		Options: &devplaneapiv1.ListInstanceTypeOptions{
			IncludeCpu:       &includeCPU,
			SkipAccessFilter: &skipAccessFilter,
		},
	}))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	mapped := mapProtoInstanceTypesToGPUSearchResponse(res.Msg.GetItems())
	return &gpusearch.InstanceTypesResponse{Items: mapped.AllInstanceTypes}, nil
}

// GetAllInstanceTypesWithCloudCreds fetches org-scoped instance types from dev-plane's public Connect API.
func (s AuthHTTPStore) GetAllInstanceTypesWithCloudCreds(orgID string) (*gpusearch.AllInstanceTypesResponse, error) {
	client := devplaneapiv1connect.NewInstanceServiceClient(
		&http.Client{Transport: &authHTTPStoreTransport{store: &s, base: http.DefaultTransport}},
		config.NewConstants().GetBrevPublicAPIURL(),
	)
	includeUnavailable := false
	includePreemptible := false
	includeCPU := true
	uniqueInstanceType := true
	skipAccessFilter := false
	res, err := client.ListOrganizationAvailableInstanceTypes(context.Background(), connect.NewRequest(&devplaneapiv1.ListOrganizationAvailableInstanceTypesRequest{
		OrganizationId: orgID,
		Options: &devplaneapiv1.ListInstanceTypeOptions{
			IncludeUnavailable: &includeUnavailable,
			IncludePreemptible: &includePreemptible,
			IncludeCpu:         &includeCPU,
			UniqueInstanceType: &uniqueInstanceType,
			SkipAccessFilter:   &skipAccessFilter,
		},
	}))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return mapProtoInstanceTypesToGPUSearchResponse(res.Msg.GetItems()), nil
}

func mapProtoInstanceTypesToGPUSearchResponse(instanceTypes []*devplaneapiv1.InstanceType) *gpusearch.AllInstanceTypesResponse {
	items := make([]gpusearch.InstanceType, 0, len(instanceTypes))
	for _, instanceType := range instanceTypes {
		if instanceType == nil {
			continue
		}
		item := gpusearch.InstanceType{
			Type:                   instanceType.GetType(),
			SupportedArchitectures: instanceType.GetSupportedArchitectures(),
			Memory:                 instanceType.GetMemory(),
			InstanceMemoryBytes:    mapProtoBytes(instanceType.GetMemoryBytes()),
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
			item.BasePrice = mapProtoCurrencyAmount(basePrice)
		}
		for _, gpu := range instanceType.GetSupportedGpus() {
			if gpu == nil {
				continue
			}
			item.SupportedGPUs = append(item.SupportedGPUs, gpusearch.GPU{
				Count:        int(gpu.GetCount()),
				Name:         gpu.GetName(),
				Manufacturer: gpu.GetManufacturer(),
				Memory:       gpu.GetMemory(),
				MemoryBytes:  mapProtoBytes(gpu.GetMemoryBytes()),
			})
		}
		for _, storage := range instanceType.GetSupportedStorage() {
			if storage == nil {
				continue
			}
			item.SupportedStorage = append(item.SupportedStorage, gpusearch.Storage{
				Count:        int(storage.GetCount()),
				Size:         storage.GetSize(),
				Type:         storage.GetType(),
				MinSize:      storage.GetMinSize(),
				MaxSize:      storage.GetMaxSize(),
				SizeBytes:    mapProtoBytes(storage.GetSizeBytes()),
				PricePerGBHr: mapProtoCurrencyAmount(storage.GetPricePerGbHr()),
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
		}
		items = append(items, item)
	}
	return &gpusearch.AllInstanceTypesResponse{AllInstanceTypes: items}
}

func mapProtoBytes(value *devplaneapiv1.Bytes) gpusearch.MemoryBytes {
	if value == nil {
		return gpusearch.MemoryBytes{}
	}
	return gpusearch.MemoryBytes{Value: value.GetValue(), Unit: value.GetUnit()}
}

func mapProtoCurrencyAmount(value *devplaneapiv1.CurrencyAmount) gpusearch.BasePrice {
	if value == nil {
		return gpusearch.BasePrice{}
	}
	return gpusearch.BasePrice{Currency: value.GetCurrency(), Amount: value.GetAmount()}
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
