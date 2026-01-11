package gpusearch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockGPUSearchStore is a mock implementation of GPUSearchStore for testing
type MockGPUSearchStore struct {
	Response *InstanceTypesResponse
	Err      error
}

func (m *MockGPUSearchStore) GetInstanceTypes() (*InstanceTypesResponse, error) {
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
	instances := processInstances(response.Items)

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
	instances := processInstances(response.Items)

	// Filter by A10G
	filtered := filterInstances(instances, "A10G", 0, 0, 0)
	assert.Len(t, filtered, 2, "Should have 2 A10G instances")

	// Filter by V100
	filtered = filterInstances(instances, "V100", 0, 0, 0)
	assert.Len(t, filtered, 2, "Should have 2 V100 instances")

	// Filter by lowercase (case-insensitive)
	filtered = filterInstances(instances, "v100", 0, 0, 0)
	assert.Len(t, filtered, 2, "Should have 2 V100 instances (case-insensitive)")

	// Filter by partial match
	filtered = filterInstances(instances, "A1", 0, 0, 0)
	assert.Len(t, filtered, 3, "Should have 3 instances matching 'A1' (A10G and A100)")
}

func TestFilterInstancesByMinVRAM(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Filter by min VRAM 24GB
	filtered := filterInstances(instances, "", 24, 0, 0)
	assert.Len(t, filtered, 4, "Should have 4 instances with >= 24GB VRAM")

	// Filter by min VRAM 40GB
	filtered = filterInstances(instances, "", 40, 0, 0)
	assert.Len(t, filtered, 1, "Should have 1 instance with >= 40GB VRAM")
	assert.Equal(t, "A100", filtered[0].GPUName)
}

func TestFilterInstancesByMinTotalVRAM(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Filter by min total VRAM 60GB
	filtered := filterInstances(instances, "", 0, 60, 0)
	assert.Len(t, filtered, 2, "Should have 2 instances with >= 60GB total VRAM")

	// Filter by min total VRAM 300GB
	filtered = filterInstances(instances, "", 0, 300, 0)
	assert.Len(t, filtered, 1, "Should have 1 instance with >= 300GB total VRAM")
	assert.Equal(t, "p4d.24xlarge", filtered[0].Type)
}

func TestFilterInstancesByMinCapability(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Filter by capability >= 8.0
	filtered := filterInstances(instances, "", 0, 0, 8.0)
	assert.Len(t, filtered, 4, "Should have 4 instances with capability >= 8.0")

	// Filter by capability >= 8.5
	filtered = filterInstances(instances, "", 0, 0, 8.5)
	assert.Len(t, filtered, 3, "Should have 3 instances with capability >= 8.5")
}

func TestFilterInstancesCombined(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Filter by GPU name and min VRAM
	filtered := filterInstances(instances, "A10G", 24, 0, 0)
	assert.Len(t, filtered, 2, "Should have 2 A10G instances with >= 24GB VRAM")

	// Filter by GPU name, min VRAM, and capability
	filtered = filterInstances(instances, "", 24, 0, 8.5)
	assert.Len(t, filtered, 3, "Should have 3 instances with >= 24GB VRAM and capability >= 8.5")
}

func TestSortInstancesByPrice(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Sort by price ascending
	sortInstances(instances, "price", false)
	assert.Equal(t, "g4dn.xlarge", instances[0].Type, "Cheapest should be g4dn.xlarge")
	assert.Equal(t, "p4d.24xlarge", instances[len(instances)-1].Type, "Most expensive should be p4d.24xlarge")

	// Sort by price descending
	sortInstances(instances, "price", true)
	assert.Equal(t, "p4d.24xlarge", instances[0].Type, "Most expensive should be first when descending")
}

func TestSortInstancesByGPUCount(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Sort by GPU count ascending
	sortInstances(instances, "gpu-count", false)
	assert.Equal(t, 1, instances[0].GPUCount, "Instances with 1 GPU should be first")

	// Sort by GPU count descending
	sortInstances(instances, "gpu-count", true)
	assert.Equal(t, 8, instances[0].GPUCount, "Instance with 8 GPUs should be first when descending")
}

func TestSortInstancesByVRAM(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Sort by VRAM ascending
	sortInstances(instances, "vram", false)
	assert.Equal(t, 16.0, instances[0].VRAMPerGPU, "Instances with 16GB VRAM should be first")

	// Sort by VRAM descending
	sortInstances(instances, "vram", true)
	assert.Equal(t, 40.0, instances[0].VRAMPerGPU, "Instance with 40GB VRAM should be first when descending")
}

func TestSortInstancesByTotalVRAM(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Sort by total VRAM ascending
	sortInstances(instances, "total-vram", false)
	assert.Equal(t, 16.0, instances[0].TotalVRAM, "Instances with 16GB total VRAM should be first")

	// Sort by total VRAM descending
	sortInstances(instances, "total-vram", true)
	assert.Equal(t, 320.0, instances[0].TotalVRAM, "Instance with 320GB total VRAM should be first when descending")
}

func TestSortInstancesByVCPU(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Sort by vCPU ascending
	sortInstances(instances, "vcpu", false)
	assert.Equal(t, 4, instances[0].VCPUs, "Instances with 4 vCPUs should be first")

	// Sort by vCPU descending
	sortInstances(instances, "vcpu", true)
	assert.Equal(t, 96, instances[0].VCPUs, "Instance with 96 vCPUs should be first when descending")
}

func TestSortInstancesByCapability(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Sort by capability ascending
	sortInstances(instances, "capability", false)
	assert.Equal(t, 7.0, instances[0].Capability, "Instances with capability 7.0 should be first")

	// Sort by capability descending
	sortInstances(instances, "capability", true)
	assert.Equal(t, 8.9, instances[0].Capability, "Instance with capability 8.9 should be first when descending")
}

func TestSortInstancesByType(t *testing.T) {
	response := createTestInstanceTypes()
	instances := processInstances(response.Items)

	// Sort by type ascending
	sortInstances(instances, "type", false)
	assert.Equal(t, "g4dn.xlarge", instances[0].Type, "g4dn.xlarge should be first alphabetically")

	// Sort by type descending
	sortInstances(instances, "type", true)
	assert.Equal(t, "p4d.24xlarge", instances[0].Type, "p4d.24xlarge should be first when descending")
}

func TestEmptyInstanceTypes(t *testing.T) {
	response := &InstanceTypesResponse{Items: []InstanceType{}}
	instances := processInstances(response.Items)

	assert.Len(t, instances, 0, "Should have 0 instances")

	filtered := filterInstances(instances, "A100", 0, 0, 0)
	assert.Len(t, filtered, 0, "Filtered should also be empty")
}

func TestNonGPUInstancesAreFiltered(t *testing.T) {
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

	instances := processInstances(response.Items)
	assert.Len(t, instances, 1, "Should only have 1 GPU instance, non-GPU instances should be filtered")
	assert.Equal(t, "g5.xlarge", instances[0].Type)
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

	instances := processInstances(response.Items)
	assert.Len(t, instances, 1)
	assert.Equal(t, 24.0, instances[0].VRAMPerGPU, "Should fall back to MemoryBytes when Memory string is empty")
}
