package gpusearch

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/stretchr/testify/assert"
)

// Mock data for testing
var testInstances = []entity.GPUInstanceType{
	{
		InstanceType:     "p4d.24xlarge",
		GPUName:          "NVIDIA A100",
		GPUMemoryGB:      40,
		GPUCount:         8,
		TotalGPUMemoryGB: 320,
		GPUCapability:    8.0,
		VCPUs:            96,
		MemoryGB:         1152,
		PricePerHour:     32.77,
		Provider:         "aws",
		Available:        true,
	},
	{
		InstanceType:     "p3.2xlarge",
		GPUName:          "NVIDIA V100",
		GPUMemoryGB:      16,
		GPUCount:         1,
		TotalGPUMemoryGB: 16,
		GPUCapability:    7.0,
		VCPUs:            8,
		MemoryGB:         61,
		PricePerHour:     3.06,
		Provider:         "aws",
		Available:        true,
	},
	{
		InstanceType:     "g5.xlarge",
		GPUName:          "NVIDIA A10G",
		GPUMemoryGB:      24,
		GPUCount:         1,
		TotalGPUMemoryGB: 24,
		GPUCapability:    8.6,
		VCPUs:            4,
		MemoryGB:         16,
		PricePerHour:     1.01,
		Provider:         "aws",
		Available:        false,
	},
	{
		InstanceType:     "a2-highgpu-1g",
		GPUName:          "NVIDIA A100",
		GPUMemoryGB:      40,
		GPUCount:         1,
		TotalGPUMemoryGB: 40,
		GPUCapability:    8.0,
		VCPUs:            12,
		MemoryGB:         85,
		PricePerHour:     3.67,
		Provider:         "gcp",
		Available:        true,
	},
	{
		InstanceType:     "h100-80gb-1",
		GPUName:          "NVIDIA H100",
		GPUMemoryGB:      80,
		GPUCount:         1,
		TotalGPUMemoryGB: 80,
		GPUCapability:    9.0,
		VCPUs:            16,
		MemoryGB:         128,
		PricePerHour:     5.50,
		Provider:         "gcp",
		Available:        true,
	},
}

func TestFilterByGPUName(t *testing.T) {
	opts := GPUSearchOptions{GPUName: "a100"}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, 2)
	for _, inst := range filtered {
		assert.Contains(t, inst.GPUName, "A100")
	}
}

func TestFilterByGPUNameCaseInsensitive(t *testing.T) {
	opts := GPUSearchOptions{GPUName: "H100"}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, 1)
	assert.Equal(t, "NVIDIA H100", filtered[0].GPUName)
}

func TestFilterByGPUMemory(t *testing.T) {
	opts := GPUSearchOptions{GPUMemory: 40}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, 3)
	for _, inst := range filtered {
		assert.GreaterOrEqual(t, inst.GPUMemoryGB, 40.0)
	}
}

func TestFilterByTotalVRAM(t *testing.T) {
	opts := GPUSearchOptions{TotalVRAM: 80}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, 2)
	for _, inst := range filtered {
		assert.GreaterOrEqual(t, inst.TotalGPUMemoryGB, 80.0)
	}
}

func TestFilterByCapability(t *testing.T) {
	opts := GPUSearchOptions{Capability: 8.5}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, 2)
	for _, inst := range filtered {
		assert.GreaterOrEqual(t, inst.GPUCapability, 8.5)
	}
}

func TestFilterByProvider(t *testing.T) {
	opts := GPUSearchOptions{Provider: "gcp"}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, 2)
	for _, inst := range filtered {
		assert.Equal(t, "gcp", inst.Provider)
	}
}

func TestFilterByProviderCaseInsensitive(t *testing.T) {
	opts := GPUSearchOptions{Provider: "AWS"}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, 3)
	for _, inst := range filtered {
		assert.Equal(t, "aws", inst.Provider)
	}
}

func TestFilterByAvailableOnly(t *testing.T) {
	opts := GPUSearchOptions{AvailableOnly: true}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, 4)
	for _, inst := range filtered {
		assert.True(t, inst.Available)
	}
}

func TestCombinedFilters(t *testing.T) {
	opts := GPUSearchOptions{
		GPUName:       "a100",
		GPUMemory:     40,
		Provider:      "aws",
		AvailableOnly: true,
	}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, 1)
	assert.Equal(t, "p4d.24xlarge", filtered[0].InstanceType)
}

func TestSortByInstanceType(t *testing.T) {
	instances := make([]entity.GPUInstanceType, len(testInstances))
	copy(instances, testInstances)

	sortInstances(instances, "instance-type", false)

	assert.Equal(t, "a2-highgpu-1g", instances[0].InstanceType)
	assert.Equal(t, "p4d.24xlarge", instances[len(instances)-1].InstanceType)
}

func TestSortByGPUName(t *testing.T) {
	instances := make([]entity.GPUInstanceType, len(testInstances))
	copy(instances, testInstances)

	sortInstances(instances, "gpu-name", false)

	assert.Equal(t, "NVIDIA A100", instances[0].GPUName)
	assert.Equal(t, "NVIDIA V100", instances[len(instances)-1].GPUName)
}

func TestSortByPrice(t *testing.T) {
	instances := make([]entity.GPUInstanceType, len(testInstances))
	copy(instances, testInstances)

	sortInstances(instances, "price", false)

	assert.Equal(t, 1.01, instances[0].PricePerHour)
	assert.Equal(t, 32.77, instances[len(instances)-1].PricePerHour)
}

func TestSortByPriceDescending(t *testing.T) {
	instances := make([]entity.GPUInstanceType, len(testInstances))
	copy(instances, testInstances)

	sortInstances(instances, "price", true)

	assert.Equal(t, 32.77, instances[0].PricePerHour)
	assert.Equal(t, 1.01, instances[len(instances)-1].PricePerHour)
}

func TestSortByGPUMemory(t *testing.T) {
	instances := make([]entity.GPUInstanceType, len(testInstances))
	copy(instances, testInstances)

	sortInstances(instances, "gpu-memory", false)

	assert.Equal(t, 16.0, instances[0].GPUMemoryGB)
	assert.Equal(t, 80.0, instances[len(instances)-1].GPUMemoryGB)
}

func TestSortByTotalVRAM(t *testing.T) {
	instances := make([]entity.GPUInstanceType, len(testInstances))
	copy(instances, testInstances)

	sortInstances(instances, "total-vram", false)

	assert.Equal(t, 16.0, instances[0].TotalGPUMemoryGB)
	assert.Equal(t, 320.0, instances[len(instances)-1].TotalGPUMemoryGB)
}

func TestSortByCapability(t *testing.T) {
	instances := make([]entity.GPUInstanceType, len(testInstances))
	copy(instances, testInstances)

	sortInstances(instances, "capability", false)

	assert.Equal(t, 7.0, instances[0].GPUCapability)
	assert.Equal(t, 9.0, instances[len(instances)-1].GPUCapability)
}

func TestSortByGPUCount(t *testing.T) {
	instances := make([]entity.GPUInstanceType, len(testInstances))
	copy(instances, testInstances)

	sortInstances(instances, "gpu-count", false)

	assert.Equal(t, 1, instances[0].GPUCount)
	assert.Equal(t, 8, instances[len(instances)-1].GPUCount)
}

func TestFormatMemory(t *testing.T) {
	assert.Equal(t, "40GB", formatMemory(40))
	assert.Equal(t, "16.5GB", formatMemory(16.5))
	assert.Equal(t, "-", formatMemory(0))
}

func TestFormatCapability(t *testing.T) {
	assert.Equal(t, "8.0", formatCapability(8.0))
	assert.Equal(t, "7.5", formatCapability(7.5))
	assert.Equal(t, "-", formatCapability(0))
}

func TestFormatPrice(t *testing.T) {
	assert.Equal(t, "$3.06", formatPrice(3.06))
	assert.Equal(t, "$32.77", formatPrice(32.77))
	assert.Equal(t, "-", formatPrice(0))
}

func TestEmptyFilter(t *testing.T) {
	opts := GPUSearchOptions{}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, len(testInstances))
}

func TestNoMatchingFilter(t *testing.T) {
	opts := GPUSearchOptions{GPUName: "nonexistent"}
	filtered := filterInstances(testInstances, opts)

	assert.Len(t, filtered, 0)
}
