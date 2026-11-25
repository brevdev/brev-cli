package gpusearch

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockGPUSearchStore is a mock implementation of GPUSearchStore for testing
type MockGPUSearchStore struct {
	Response *InstanceTypesResponse
	Err      error
}

func (m *MockGPUSearchStore) GetInstanceTypes(_ bool) (*InstanceTypesResponse, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Response, nil
}

func createTestInstanceTypes() *InstanceTypesResponse {
	return &InstanceTypesResponse{
		Items: []InstanceType{
			{
				Type: "g5.xlarge",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "A10G", Manufacturer: "NVIDIA", Memory: "24GiB"},
				},
				Memory:    "16GiB",
				VCPU:      4,
				BasePrice: BasePrice{Currency: "USD", Amount: "1.006"},
			},
			{
				Type: "g5.2xlarge",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "A10G", Manufacturer: "NVIDIA", Memory: "24GiB"},
				},
				Memory:    "32GiB",
				VCPU:      8,
				BasePrice: BasePrice{Currency: "USD", Amount: "1.212"},
			},
			{
				Type: "p3.2xlarge",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "V100", Manufacturer: "NVIDIA", Memory: "16GiB"},
				},
				Memory:    "61GiB",
				VCPU:      8,
				BasePrice: BasePrice{Currency: "USD", Amount: "3.06"},
			},
			{
				Type: "p3.8xlarge",
				SupportedGPUs: []GPU{
					{Count: 4, Name: "V100", Manufacturer: "NVIDIA", Memory: "16GiB"},
				},
				Memory:    "244GiB",
				VCPU:      32,
				BasePrice: BasePrice{Currency: "USD", Amount: "12.24"},
			},
			{
				Type: "p4d.24xlarge",
				SupportedGPUs: []GPU{
					{Count: 8, Name: "A100", Manufacturer: "NVIDIA", Memory: "40GiB"},
				},
				Memory:    "1152GiB",
				VCPU:      96,
				BasePrice: BasePrice{Currency: "USD", Amount: "32.77"},
			},
			{
				Type: "g4dn.xlarge",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "T4", Manufacturer: "NVIDIA", Memory: "16GiB"},
				},
				Memory:    "16GiB",
				VCPU:      4,
				BasePrice: BasePrice{Currency: "USD", Amount: "0.526"},
			},
			{
				Type: "g6.xlarge",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "L4", Manufacturer: "NVIDIA", Memory: "24GiB"},
				},
				Memory:    "16GiB",
				VCPU:      4,
				BasePrice: BasePrice{Currency: "USD", Amount: "0.805"},
			},
		},
	}
}

func TestParseMemoryToGB(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"Simple GiB", "24GiB", 24},
		{"GiB with MiB", "22GiB360MiB", 22.3515625},
		{"Simple GB", "16GB", 16},
		{"Large GiB", "1152GiB", 1152},
		{"Empty string", "", 0},
		{"MiB only", "512MiB", 0.5},
		{"With spaces", "24 GiB", 24},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMemoryToGB(tt.input)
			assert.InDelta(t, tt.expected, result, 0.01, "Memory parsing failed for %s", tt.input)
		})
	}
}

func TestGetGPUCapability(t *testing.T) {
	tests := []struct {
		name     string
		gpuName  string
		expected float64
	}{
		{"A100", "A100", 8.0},
		{"A10G", "A10G", 8.6},
		{"V100", "V100", 7.0},
		{"T4", "T4", 7.5},
		{"L4", "L4", 8.9},
		{"L40S", "L40S", 8.9},
		{"H100", "H100", 9.0},
		{"Unknown GPU", "Unknown", 0},
		{"Case insensitive", "a100", 8.0},
		{"Gaudi", "HL-205", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getGPUCapability(tt.gpuName)
			assert.Equal(t, tt.expected, result, "GPU capability mismatch for %s", tt.gpuName)
		})
	}
}

func TestProcessInstances(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	assert.Len(t, instances, 7, "Expected 7 GPU instances")

	// Check specific instance
	var a10gInstance *GPUInstanceInfo
	for i := range instances {
		if instances[i].Type == "g5.xlarge" {
			a10gInstance = &instances[i]
			break
		}
	}

	assert.NotNil(t, a10gInstance, "g5.xlarge instance should exist")
	assert.Equal(t, "A10G", a10gInstance.GPUName)
	assert.Equal(t, 1, a10gInstance.GPUCount)
	assert.Equal(t, 24.0, a10gInstance.VRAMPerGPU)
	assert.Equal(t, 24.0, a10gInstance.TotalVRAM)
	assert.Equal(t, 8.6, a10gInstance.Capability)
	assert.Equal(t, 4, a10gInstance.VCPUs)
	assert.InDelta(t, 1.006, a10gInstance.PricePerHour, 0.001)
}

func TestFilterInstancesByGPUName(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Filter by A10G
	filtered := FilterInstances(instances, "A10G", "", "", 0, 0, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 2, "Should have 2 A10G instances")

	// Filter by V100
	filtered = FilterInstances(instances, "V100", "", "", 0, 0, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 2, "Should have 2 V100 instances")

	// Filter by lowercase (case-insensitive)
	filtered = FilterInstances(instances, "v100", "", "", 0, 0, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 2, "Should have 2 V100 instances (case-insensitive)")

	// Filter by partial match
	filtered = FilterInstances(instances, "A1", "", "", 0, 0, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 3, "Should have 3 instances matching 'A1' (A10G and A100)")
}

func TestFilterInstancesByMinVRAM(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Filter by min VRAM 24GB
	filtered := FilterInstances(instances, "", "", "", 24, 0, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 4, "Should have 4 instances with >= 24GB VRAM")

	// Filter by min VRAM 40GB
	filtered = FilterInstances(instances, "", "", "", 40, 0, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 1, "Should have 1 instance with >= 40GB VRAM")
	assert.Equal(t, "A100", filtered[0].GPUName)
}

func TestFilterInstancesByMinTotalVRAM(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Filter by min total VRAM 60GB
	filtered := FilterInstances(instances, "", "", "", 0, 60, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 2, "Should have 2 instances with >= 60GB total VRAM")

	// Filter by min total VRAM 300GB
	filtered = FilterInstances(instances, "", "", "", 0, 300, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 1, "Should have 1 instance with >= 300GB total VRAM")
	assert.Equal(t, "p4d.24xlarge", filtered[0].Type)
}

func TestFilterInstancesByMinCapability(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Filter by capability >= 8.0
	filtered := FilterInstances(instances, "", "", "", 0, 0, 8.0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 4, "Should have 4 instances with capability >= 8.0")

	// Filter by capability >= 8.5
	filtered = FilterInstances(instances, "", "", "", 0, 0, 8.5, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 3, "Should have 3 instances with capability >= 8.5")
}

func TestFilterInstancesCombined(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Filter by GPU name and min VRAM
	filtered := FilterInstances(instances, "A10G", "", "", 24, 0, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 2, "Should have 2 A10G instances with >= 24GB VRAM")

	// Filter by GPU name, min VRAM, and capability
	filtered = FilterInstances(instances, "", "", "", 24, 0, 8.5, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 3, "Should have 3 instances with >= 24GB VRAM and capability >= 8.5")
}

func TestSortInstancesByPrice(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Sort by price ascending
	SortInstances(instances, "price", false)
	assert.Equal(t, "g4dn.xlarge", instances[0].Type, "Cheapest should be g4dn.xlarge")
	assert.Equal(t, "p4d.24xlarge", instances[len(instances)-1].Type, "Most expensive should be p4d.24xlarge")

	// Sort by price descending
	SortInstances(instances, "price", true)
	assert.Equal(t, "p4d.24xlarge", instances[0].Type, "Most expensive should be first when descending")
}

func TestSortInstancesByGPUCount(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Sort by GPU count ascending
	SortInstances(instances, "gpu-count", false)
	assert.Equal(t, 1, instances[0].GPUCount, "Instances with 1 GPU should be first")

	// Sort by GPU count descending
	SortInstances(instances, "gpu-count", true)
	assert.Equal(t, 8, instances[0].GPUCount, "Instance with 8 GPUs should be first when descending")
}

func TestSortInstancesByVRAM(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Sort by VRAM ascending
	SortInstances(instances, "vram", false)
	assert.Equal(t, 16.0, instances[0].VRAMPerGPU, "Instances with 16GB VRAM should be first")

	// Sort by VRAM descending
	SortInstances(instances, "vram", true)
	assert.Equal(t, 40.0, instances[0].VRAMPerGPU, "Instance with 40GB VRAM should be first when descending")
}

func TestSortInstancesByTotalVRAM(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Sort by total VRAM ascending
	SortInstances(instances, "total-vram", false)
	assert.Equal(t, 16.0, instances[0].TotalVRAM, "Instances with 16GB total VRAM should be first")

	// Sort by total VRAM descending
	SortInstances(instances, "total-vram", true)
	assert.Equal(t, 320.0, instances[0].TotalVRAM, "Instance with 320GB total VRAM should be first when descending")
}

func TestSortInstancesByVCPU(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Sort by vCPU ascending
	SortInstances(instances, "vcpu", false)
	assert.Equal(t, 4, instances[0].VCPUs, "Instances with 4 vCPUs should be first")

	// Sort by vCPU descending
	SortInstances(instances, "vcpu", true)
	assert.Equal(t, 96, instances[0].VCPUs, "Instance with 96 vCPUs should be first when descending")
}

func TestSortInstancesByCapability(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Sort by capability ascending
	SortInstances(instances, "capability", false)
	assert.Equal(t, 7.0, instances[0].Capability, "Instances with capability 7.0 should be first")

	// Sort by capability descending
	SortInstances(instances, "capability", true)
	assert.Equal(t, 8.9, instances[0].Capability, "Instance with capability 8.9 should be first when descending")
}

func TestSortInstancesByType(t *testing.T) {
	response := createTestInstanceTypes()
	instances := ProcessInstances(response.Items)

	// Sort by type ascending
	SortInstances(instances, "type", false)
	assert.Equal(t, "g4dn.xlarge", instances[0].Type, "g4dn.xlarge should be first alphabetically")

	// Sort by type descending
	SortInstances(instances, "type", true)
	assert.Equal(t, "p4d.24xlarge", instances[0].Type, "p4d.24xlarge should be first when descending")
}

func TestEmptyInstanceTypes(t *testing.T) {
	response := &InstanceTypesResponse{Items: []InstanceType{}}
	instances := ProcessInstances(response.Items)

	assert.Len(t, instances, 0, "Should have 0 instances")

	filtered := FilterInstances(instances, "A100", "", "", 0, 0, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 0, "Filtered should also be empty")
}

func TestNonGPUInstancesAreIncludedInProcessing(t *testing.T) {
	response := &InstanceTypesResponse{
		Items: []InstanceType{
			{
				Type:          "m5.xlarge",
				SupportedGPUs: []GPU{}, // No GPUs
				Memory:        "16GiB",
				VCPU:          4,
				BasePrice:     BasePrice{Currency: "USD", Amount: "0.192"},
			},
			{
				Type: "g5.xlarge",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "A10G", Manufacturer: "NVIDIA", Memory: "24GiB"},
				},
				Memory:    "16GiB",
				VCPU:      4,
				BasePrice: BasePrice{Currency: "USD", Amount: "1.006"},
			},
		},
	}

	instances := ProcessInstances(response.Items)
	assert.Len(t, instances, 2, "Should include both CPU and GPU instances")
	assert.Equal(t, "m5.xlarge", instances[0].Type)
	assert.Equal(t, "cpu", instances[0].Manufacturer)
	assert.Equal(t, "-", instances[0].GPUName)
	assert.Equal(t, "g5.xlarge", instances[1].Type)
}

func TestNonGPUInstancesFilteredByDefault(t *testing.T) {
	response := &InstanceTypesResponse{
		Items: []InstanceType{
			{
				Type:          "m5.xlarge",
				SupportedGPUs: []GPU{}, // No GPUs
				Memory:        "16GiB",
				VCPU:          4,
				BasePrice:     BasePrice{Currency: "USD", Amount: "0.192"},
			},
			{
				Type: "g5.xlarge",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "A10G", Manufacturer: "NVIDIA", Memory: "24GiB"},
				},
				Memory:    "16GiB",
				VCPU:      4,
				BasePrice: BasePrice{Currency: "USD", Amount: "1.006"},
			},
		},
	}

	instances := ProcessInstances(response.Items)

	// gpuOnly=true should filter out CPU instances
	filtered := FilterInstances(instances, "", "", "", 0, 0, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, filtered, 1, "gpuOnly should exclude CPU instances")
	assert.Equal(t, "g5.xlarge", filtered[0].Type)

	// gpuOnly=false should keep CPU instances
	filtered = FilterInstances(instances, "", "", "", 0, 0, 0, 0, 0, 0, 0, false, false, false, false)
	assert.Len(t, filtered, 2, "Without gpuOnly, both CPU and GPU instances pass")
}

func TestMemoryBytesAsFallback(t *testing.T) {
	response := &InstanceTypesResponse{
		Items: []InstanceType{
			{
				Type: "test.xlarge",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "TestGPU", Manufacturer: "NVIDIA", Memory: "", MemoryBytes: MemoryBytes{Value: 24576, Unit: "MiB"}}, // 24GB in MiB
				},
				Memory:    "16GiB",
				VCPU:      4,
				BasePrice: BasePrice{Currency: "USD", Amount: "1.00"},
			},
		},
	}

	instances := ProcessInstances(response.Items)
	assert.Len(t, instances, 1)
	assert.Equal(t, 24.0, instances[0].VRAMPerGPU, "Should fall back to MemoryBytes when Memory string is empty")
}

func TestFilterByMaxBootTimeExcludesUnknown(t *testing.T) {
	response := &InstanceTypesResponse{
		Items: []InstanceType{
			{
				Type: "fast-boot",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "A100", Manufacturer: "NVIDIA", Memory: "40GiB"},
				},
				Memory:              "64GiB",
				VCPU:                8,
				BasePrice:           BasePrice{Currency: "USD", Amount: "3.00"},
				EstimatedDeployTime: "5m0s",
			},
			{
				Type: "slow-boot",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "A100", Manufacturer: "NVIDIA", Memory: "40GiB"},
				},
				Memory:              "64GiB",
				VCPU:                8,
				BasePrice:           BasePrice{Currency: "USD", Amount: "2.50"},
				EstimatedDeployTime: "15m0s",
			},
			{
				Type: "unknown-boot",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "A100", Manufacturer: "NVIDIA", Memory: "40GiB"},
				},
				Memory:              "64GiB",
				VCPU:                8,
				BasePrice:           BasePrice{Currency: "USD", Amount: "2.00"},
				EstimatedDeployTime: "", // Unknown boot time
			},
		},
	}

	instances := ProcessInstances(response.Items)
	assert.Len(t, instances, 3, "Should have 3 instances before filtering")

	// Filter by max boot time of 10 minutes - should exclude unknown and slow-boot
	filtered := FilterInstances(instances, "", "", "", 0, 0, 0, 0, 0, 0, 10, false, false, false, true)
	assert.Len(t, filtered, 1, "Should have 1 instance with boot time <= 10 minutes")
	assert.Equal(t, "fast-boot", filtered[0].Type, "Only fast-boot should match")

	// Verify unknown boot time instance is excluded
	for _, inst := range filtered {
		assert.NotEqual(t, "unknown-boot", inst.Type, "Unknown boot time should be excluded")
		assert.NotEqual(t, 0, inst.BootTime, "Instances with 0 boot time should be excluded")
	}

	// Without filter, all instances should be included
	noFilter := FilterInstances(instances, "", "", "", 0, 0, 0, 0, 0, 0, 0, false, false, false, true)
	assert.Len(t, noFilter, 3, "Without filter, all 3 instances should be included")
}

func TestExtractCloud(t *testing.T) {
	tests := []struct {
		name          string
		instanceType  string
		provider      string
		expectedCloud string
	}{
		// Direct providers - cloud equals provider
		{"AWS direct", "g5.xlarge", "aws", "aws"},
		{"GCP direct", "n1-highmem-8:nvidia-tesla-v100:8", "gcp", "gcp"},
		{"Nebius direct", "gpu-h100-sxm.1gpu-16vcpu-200gb", "nebius", "nebius"},
		{"OCI direct", "oci.h100x8.sxm", "oci", "oci"},
		{"Lambda Labs direct", "gpu_1x_h100_sxm5", "lambda-labs", "lambda-labs"},
		{"Crusoe direct", "l40s-48gb.1x", "crusoe", "crusoe"},
		{"Launchpad direct", "dmz.h100x2.pcie", "launchpad", "launchpad"},

		// Aggregators - extract cloud from type name prefix
		{"Shadeform hyperstack", "hyperstack_H100", "shadeform", "hyperstack"},
		{"Shadeform latitude", "latitude_H100x4", "shadeform", "latitude"},
		{"Shadeform cudo", "cudo_A40", "shadeform", "cudo"},
		{"Shadeform horizon", "horizon_H100x8", "shadeform", "horizon"},
		{"Shadeform paperspace", "paperspace_H100", "shadeform", "paperspace"},

		// Edge cases
		{"Unknown aggregator no underscore", "someinstance", "unknown-agg", "unknown-agg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCloud(tt.instanceType, tt.provider)
			assert.Equal(t, tt.expectedCloud, result)
		})
	}
}

func TestExtractDiskInfoWithPricing(t *testing.T) {
	// Test flexible storage with pricing
	storageWithPrice := []Storage{
		{
			MinSize:      "50GiB",
			MaxSize:      "2560GiB",
			PricePerGBHr: BasePrice{Currency: "USD", Amount: "0.00014"},
		},
	}

	minGB, maxGB, pricePerMo := extractDiskInfo(storageWithPrice)
	assert.Equal(t, 50.0, minGB)
	assert.Equal(t, 2560.0, maxGB)
	assert.InDelta(t, 0.1022, pricePerMo, 0.001, "Price should be ~$0.10/GB/mo (0.00014 * 730)")

	// Test fixed storage (no pricing)
	fixedStorage := []Storage{
		{
			Size: "500GiB",
		},
	}

	minGB, maxGB, pricePerMo = extractDiskInfo(fixedStorage)
	assert.Equal(t, 500.0, minGB)
	assert.Equal(t, 500.0, maxGB)
	assert.Equal(t, 0.0, pricePerMo, "Fixed storage should have no separate price")

	// Test empty storage
	minGB, maxGB, pricePerMo = extractDiskInfo([]Storage{})
	assert.Equal(t, 0.0, minGB)
	assert.Equal(t, 0.0, maxGB)
	assert.Equal(t, 0.0, pricePerMo)
}

func TestProcessInstancesCloudExtraction(t *testing.T) {
	response := &InstanceTypesResponse{
		Items: []InstanceType{
			{
				Type:     "hyperstack_H100",
				Provider: "shadeform",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "H100", Manufacturer: "NVIDIA", Memory: "80GiB"},
				},
				Memory:    "180GiB",
				VCPU:      28,
				BasePrice: BasePrice{Currency: "USD", Amount: "2.28"},
			},
			{
				Type:     "gpu-h100-sxm.1gpu-16vcpu-200gb",
				Provider: "nebius",
				SupportedGPUs: []GPU{
					{Count: 1, Name: "H100", Manufacturer: "NVIDIA", Memory: "80GiB"},
				},
				Memory:    "200GiB",
				VCPU:      16,
				BasePrice: BasePrice{Currency: "USD", Amount: "3.54"},
			},
		},
	}

	instances := ProcessInstances(response.Items)
	assert.Len(t, instances, 2)

	// Shadeform instance should have cloud extracted from type name
	assert.Equal(t, "hyperstack", instances[0].Cloud)
	assert.Equal(t, "shadeform", instances[0].Provider)

	// Nebius instance should have cloud = provider
	assert.Equal(t, "nebius", instances[1].Cloud)
	assert.Equal(t, "nebius", instances[1].Provider)
}

func TestAllInstanceTypesResponseLookup(t *testing.T) {
	resp := &AllInstanceTypesResponse{
		AllInstanceTypes: []InstanceType{
			{
				Type:        "hyperstack_H100_sxm5x8",
				CloudCredID: "cc-shadeform",
			},
			{
				Type: "hyperstack_H100x8_NVLINK",
			},
			{
				Type: "verda-b300-8x",
			},
		},
	}

	t.Run("GetCloudCredID returns the cloud credential instead of the workspace group", func(t *testing.T) {
		assert.Equal(t, "cc-shadeform", resp.GetCloudCredID("hyperstack_H100_sxm5x8"))
	})

	t.Run("GetCloudCredID returns empty when no cloud credential is available", func(t *testing.T) {
		assert.Equal(t, "", resp.GetCloudCredID("verda-b300-8x"))
	})

	t.Run("GetCloudCredID returns empty for unknown type", func(t *testing.T) {
		assert.Equal(t, "", resp.GetCloudCredID("hyperstack_H100x8_one"))
	})

	t.Run("HasInstanceType is true even when groups are empty", func(t *testing.T) {
		assert.True(t, resp.HasInstanceType("hyperstack_H100_sxm5x8"))
		assert.True(t, resp.HasInstanceType("hyperstack_H100x8_NVLINK"))
		assert.True(t, resp.HasInstanceType("verda-b300-8x"))
	})

	t.Run("HasInstanceType is false for unknown type", func(t *testing.T) {
		assert.False(t, resp.HasInstanceType("hyperstack_H100x8_one"))
		assert.False(t, resp.HasInstanceType(""))
	})
}

// trackingStore is a GPUSearchStore mock that records the includeCPU flag passed to
// GetInstanceTypes so command dispatch tests can assert which search path executed.
// It returns an empty (but non-nil) response by default so RunGPUSearch/RunCPUSearch
// hit the empty-results path and return nil without rendering a table.
type trackingStore struct {
	Response *InstanceTypesResponse
	Err      error
	calls    []bool
}

func (m *trackingStore) GetInstanceTypes(includeCPU bool) (*InstanceTypesResponse, error) {
	m.calls = append(m.calls, includeCPU)
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Response != nil {
		return m.Response, nil
	}
	return &InstanceTypesResponse{Items: []InstanceType{}}, nil
}

// searchTypes runs the search command with --json and returns the listed types.
func searchTypes(t *testing.T, args ...string) ([]string, *trackingStore) {
	t.Helper()
	store := &trackingStore{Response: locationCatalog()}
	cmd := NewCmdGPUSearch(terminal.New(), store)
	cmd.SetArgs(append(args, "--json"))
	out := captureStdout(t, func() { require.NoError(t, cmd.Execute()) })
	var found []GPUInstanceInfo
	require.NoError(t, json.Unmarshal([]byte(out), &found))
	var types []string
	for _, inst := range found {
		types = append(types, inst.Type)
	}
	return types, store
}

func TestSearchCommandDispatch(t *testing.T) {
	cpuTypes := []string{"cpu-no-location", "n2d-standard-2", "m5.large"} // sorted by price
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"default search lists GPU types", nil, []string{"g5.xlarge"}},
		{"gpu subcommand lists GPU types", []string{"gpu"}, []string{"g5.xlarge"}},
		{"cpu subcommand lists CPU types", []string{"cpu"}, cpuTypes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			types, store := searchTypes(t, tc.args...)
			assert.Equal(t, tc.want, types)
			// One catalog for every command, the same one 'brev create' validates against.
			assert.Equal(t, []bool{true}, store.calls)
		})
	}
}

func TestSearchCommandRejectsUnknownPositionalTokens(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"plural cpus", []string{"cpus"}},
		{"uppercase CPU", []string{"CPU"}},
		{"uppercase GPU", []string{"GPU"}},
		{"arbitrary badcmd", []string{"badcmd"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &trackingStore{}
			cmd := NewCmdGPUSearch(terminal.New(), store)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			assert.Error(t, err)
			assert.ErrorContains(t, err, "unknown subcommand")
			assert.ErrorContains(t, err, "Available subcommands: cpu, gpu, locations")
			assert.Empty(t, store.calls, "unknown subcommand should not call the store")
		})
	}
}

// captureStdout returns what fn writes to os.Stdout. Tables and JSON are
// written there directly, and a pipe makes IsStdoutPiped report true.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

// locationCatalog mirrors the live catalog shape from issue #454: n2d-standard-2
// defaults to asia-south1 but is also available in us-west2.
func locationCatalog() *InstanceTypesResponse {
	return &InstanceTypesResponse{Items: []InstanceType{
		{
			Type: "n2d-standard-2", Provider: "gcp", VCPU: 2, Memory: "8GiB",
			BasePrice:          BasePrice{Currency: "USD", Amount: "0.07"},
			Location:           "asia-south1",
			SubLocation:        "asia-south1-a",
			AvailableLocations: []string{"asia-south1", "us-west2"},
		},
		{
			Type: "m5.large", Provider: "aws", VCPU: 2, Memory: "8GiB",
			BasePrice:          BasePrice{Currency: "USD", Amount: "0.10"},
			Location:           "us-east-1",
			AvailableLocations: []string{"us-east-1"},
		},
		{
			Type: "g5.xlarge", Provider: "aws", VCPU: 4, Memory: "16GiB",
			SupportedGPUs:      []GPU{{Count: 1, Name: "A10G", Manufacturer: "NVIDIA", Memory: "24GiB"}},
			BasePrice:          BasePrice{Currency: "USD", Amount: "1.006"},
			Location:           "us-east-1",
			AvailableLocations: []string{"us-east-1", "us-west-2"},
		},
		{Type: "cpu-no-location", Provider: "aws", VCPU: 2, Memory: "8GiB"},
	}}
}

func TestProcessInstancesCopiesLocations(t *testing.T) {
	byType := map[string]GPUInstanceInfo{}
	for _, inst := range ProcessInstances(locationCatalog().Items) {
		byType[inst.Type] = inst
	}

	cpu := byType["n2d-standard-2"]
	assert.Equal(t, "asia-south1", cpu.Location)
	assert.Equal(t, "asia-south1-a", cpu.SubLocation)
	assert.Equal(t, []string{"asia-south1", "us-west2"}, cpu.AvailableLocations)

	gpu := byType["g5.xlarge"]
	assert.Equal(t, "us-east-1", gpu.Location)
	assert.Equal(t, []string{"us-east-1", "us-west-2"}, gpu.AvailableLocations)
}

func TestFilterByLocation(t *testing.T) {
	instances := ProcessInstances(locationCatalog().Items)

	t.Run("empty location is a no-op", func(t *testing.T) {
		assert.Equal(t, instances, FilterByLocation(instances, ""))
	})

	t.Run("keeps types available in the location and reports it as their location", func(t *testing.T) {
		got := FilterByLocation(instances, "us-west2")
		require.Len(t, got, 1)
		assert.Equal(t, "n2d-standard-2", got[0].Type)
		assert.Equal(t, "us-west2", got[0].Location)
		assert.Empty(t, got[0].SubLocation, "the default sub-location belongs to the default location")
	})

	t.Run("match is exact", func(t *testing.T) {
		assert.Empty(t, FilterByLocation(instances, "us-west"))
		assert.Empty(t, FilterByLocation(instances, "US-WEST2"))
	})

	t.Run("does not modify the input", func(t *testing.T) {
		FilterByLocation(instances, "us-west2")
		for _, inst := range instances {
			if inst.Type == "n2d-standard-2" {
				assert.Equal(t, "asia-south1", inst.Location)
			}
		}
	})
}

func TestSearchCommandLocation(t *testing.T) {
	run := func(args ...string) string {
		store := &trackingStore{Response: locationCatalog()}
		cmd := NewCmdGPUSearch(terminal.New(), store)
		cmd.SetArgs(args)
		return captureStdout(t, func() { require.NoError(t, cmd.Execute()) })
	}
	types := func(out string) []string {
		var got []GPUInstanceInfo
		require.NoError(t, json.Unmarshal([]byte(out), &got))
		var names []string
		for _, inst := range got {
			names = append(names, inst.Type)
		}
		return names
	}

	t.Run("cpu JSON includes default location fields", func(t *testing.T) {
		var got []map[string]any
		require.NoError(t, json.Unmarshal([]byte(run("cpu", "--json")), &got))
		byType := map[string]map[string]any{}
		for _, inst := range got {
			byType[fmt.Sprint(inst["type"])] = inst
		}
		assert.Equal(t, "asia-south1", byType["n2d-standard-2"]["location"])
		assert.Equal(t, "asia-south1-a", byType["n2d-standard-2"]["sub_location"])
		assert.Equal(t, []any{"asia-south1", "us-west2"}, byType["n2d-standard-2"]["available_locations"])
		assert.NotContains(t, byType["cpu-no-location"], "location", "location fields are omitted when unknown")
		assert.NotContains(t, byType["cpu-no-location"], "available_locations")
	})

	t.Run("cpu --location filters", func(t *testing.T) {
		assert.Equal(t, []string{"n2d-standard-2"}, types(run("cpu", "--location", "us-west2", "--json")))
	})

	t.Run("gpu --location filters on the parent and the gpu subcommand", func(t *testing.T) {
		assert.Equal(t, []string{"g5.xlarge"}, types(run("--location", "us-west-2", "--json")))
		assert.Equal(t, []string{"g5.xlarge"}, types(run("gpu", "--location", "us-west-2", "--json")))
	})

	t.Run("no match reports empty results", func(t *testing.T) {
		assert.Equal(t, "[]\n", run("cpu", "--location", "mars-1", "--json"))
	})

	t.Run("piped cpu table keeps TYPE and TARGET_DISK first and adds LOCATION", func(t *testing.T) {
		lines := strings.Split(strings.TrimSpace(run("cpu", "--location", "us-west2")), "\n")
		require.Len(t, lines, 2)
		assert.Equal(t, []string{"TYPE", "TARGET_DISK", "PROVIDER", "LOCATION"}, strings.Fields(lines[0])[:4])
		row := strings.Fields(lines[1])
		assert.Equal(t, "n2d-standard-2", row[0])
		assert.Equal(t, "us-west2", row[3])
	})

	t.Run("piped tables show the default location, or - when unknown", func(t *testing.T) {
		locations := map[string]string{}
		for _, kind := range []string{"gpu", "cpu"} {
			lines := strings.Split(strings.TrimSpace(run(kind)), "\n")
			assert.Equal(t, []string{"TYPE", "TARGET_DISK", "PROVIDER", "LOCATION"}, strings.Fields(lines[0])[:4])
			for _, line := range lines[1:] {
				fields := strings.Fields(line)
				locations[fields[0]] = fields[3]
			}
		}
		assert.Equal(t, "us-east-1", locations["g5.xlarge"])
		assert.Equal(t, "asia-south1", locations["n2d-standard-2"])
		assert.Equal(t, "-", locations["cpu-no-location"])
	})
}

func TestLocationColumnInEveryTable(t *testing.T) {
	instances := ProcessInstances(locationCatalog().Items)
	var gpus, cpus []GPUInstanceInfo
	for _, inst := range instances {
		if inst.Manufacturer == "cpu" {
			cpus = append(cpus, inst)
		} else {
			gpus = append(gpus, inst)
		}
	}
	term := terminal.New()
	tables := map[string]func(){
		"gpu":             func() { displayGPUTable(term, gpus) },
		"gpu plain":       func() { displayGPUTablePlain(gpus) },
		"gpu wide":        func() { displayGPUTableWide(term, gpus) },
		"gpu plain wide":  func() { displayGPUTablePlainWide(gpus) },
		"cpu":             func() { displayCPUTable(cpus) },
		"cpu plain":       func() { displayCPUTablePlain(cpus) },
		"cpu no location": func() { displayCPUTablePlain(cpus[len(cpus)-1:]) },
	}
	for name, render := range tables {
		t.Run(name, func(t *testing.T) {
			out := captureStdout(t, render)
			assert.Contains(t, out, "LOCATION")
			if name == "cpu no location" {
				assert.Equal(t, "-", strings.Fields(strings.Split(strings.TrimSpace(out), "\n")[1])[3])
			} else {
				assert.Contains(t, out, "us-east-1")
			}
		})
	}
}
