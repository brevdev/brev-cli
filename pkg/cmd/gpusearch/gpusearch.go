// Package gpusearch provides a command to search and filter GPU instance types
package gpusearch

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

// MemoryBytes represents the memory size with value and unit
type MemoryBytes struct {
	Value int64  `json:"value"`
	Unit  string `json:"unit"`
}

// GPU represents a GPU configuration within an instance type
type GPU struct {
	Count        int         `json:"count"`
	Name         string      `json:"name"`
	Manufacturer string      `json:"manufacturer"`
	Memory       string      `json:"memory"`
	MemoryBytes  MemoryBytes `json:"memory_bytes"`
}

// BasePrice represents the pricing information
type BasePrice struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

// InstanceType represents an instance type from the API
type InstanceType struct {
	Type               string        `json:"type"`
	SupportedGPUs      []GPU         `json:"supported_gpus"`
	SupportedStorage   []interface{} `json:"supported_storage"` // Complex objects, not used in filtering
	Memory             string        `json:"memory"`
	VCPU               int           `json:"vcpu"`
	BasePrice          BasePrice     `json:"base_price"`
	Location           string        `json:"location"`
	SubLocation        string        `json:"sub_location"`
	AvailableLocations []string      `json:"available_locations"`
}

// InstanceTypesResponse represents the API response
type InstanceTypesResponse struct {
	Items []InstanceType `json:"items"`
}

// GPUSearchStore defines the interface for fetching instance types
type GPUSearchStore interface {
	GetInstanceTypes() (*InstanceTypesResponse, error)
}

var (
	long = `Search and filter GPU instance types available on Brev.

Filter instances by GPU name, VRAM, total VRAM, and GPU compute capability.
Sort results by various columns to find the best instance for your needs.`

	example = `
  # List all GPU instances
  brev gpu-search

  # Filter by GPU name (case-insensitive, partial match)
  brev gpu-search --gpu-name A100
  brev gpu-search --gpu-name "L40S"

  # Filter by minimum VRAM per GPU (in GB)
  brev gpu-search --min-vram 24

  # Filter by minimum total VRAM (in GB)
  brev gpu-search --min-total-vram 80

  # Filter by minimum GPU compute capability
  brev gpu-search --min-capability 8.0

  # Sort by different columns (price, gpu-count, vram, total-vram, vcpu)
  brev gpu-search --sort price
  brev gpu-search --sort total-vram --desc

  # Combine filters
  brev gpu-search --gpu-name A100 --min-vram 40 --sort price
`
)

// NewCmdGPUSearch creates the gpu-search command
func NewCmdGPUSearch(t *terminal.Terminal, store GPUSearchStore) *cobra.Command {
	var gpuName string
	var minVRAM float64
	var minTotalVRAM float64
	var minCapability float64
	var sortBy string
	var descending bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "gpu-search",
		Aliases:               []string{"gpu", "gpus", "gpu-list"},
		DisableFlagsInUseLine: true,
		Short:                 "Search and filter GPU instance types",
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunGPUSearch(t, store, gpuName, minVRAM, minTotalVRAM, minCapability, sortBy, descending)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&gpuName, "gpu-name", "g", "", "Filter by GPU name (case-insensitive, partial match)")
	cmd.Flags().Float64VarP(&minVRAM, "min-vram", "v", 0, "Minimum VRAM per GPU in GB")
	cmd.Flags().Float64VarP(&minTotalVRAM, "min-total-vram", "t", 0, "Minimum total VRAM (GPU count * VRAM) in GB")
	cmd.Flags().Float64VarP(&minCapability, "min-capability", "c", 0, "Minimum GPU compute capability (e.g., 8.0 for Ampere)")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "price", "Sort by: price, gpu-count, vram, total-vram, vcpu, type")
	cmd.Flags().BoolVarP(&descending, "desc", "d", false, "Sort in descending order")

	return cmd
}

// GPUInstanceInfo holds processed GPU instance information for display
type GPUInstanceInfo struct {
	Type         string
	GPUName      string
	GPUCount     int
	VRAMPerGPU   float64 // in GB
	TotalVRAM    float64 // in GB
	Capability   float64
	VCPUs        int
	Memory       string
	PricePerHour float64
	Manufacturer string
}

// RunGPUSearch executes the GPU search with filters and sorting
func RunGPUSearch(t *terminal.Terminal, store GPUSearchStore, gpuName string, minVRAM, minTotalVRAM, minCapability float64, sortBy string, descending bool) error {
	response, err := store.GetInstanceTypes()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if response == nil || len(response.Items) == 0 {
		t.Vprint(t.Yellow("No instance types found"))
		return nil
	}

	// Process and filter instances
	instances := processInstances(response.Items)

	// Apply filters
	filtered := filterInstances(instances, gpuName, minVRAM, minTotalVRAM, minCapability)

	if len(filtered) == 0 {
		t.Vprint(t.Yellow("No GPU instances match the specified filters"))
		return nil
	}

	// Sort instances
	sortInstances(filtered, sortBy, descending)

	// Display results
	displayGPUTable(t, filtered)

	t.Vprintf("\n%s\n", t.Green(fmt.Sprintf("Found %d GPU instance types", len(filtered))))

	return nil
}

// parseMemoryToGB converts memory string like "22GiB360MiB" or "40GiB" to GB
func parseMemoryToGB(memory string) float64 {
	// Handle memory_bytes if provided (in MiB)
	// Otherwise parse the string format

	var totalGB float64

	// Match GiB values
	gibRe := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*GiB`)
	gibMatches := gibRe.FindAllStringSubmatch(memory, -1)
	for _, match := range gibMatches {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			totalGB += val
		}
	}

	// Match MiB values and convert to GB
	mibRe := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*MiB`)
	mibMatches := mibRe.FindAllStringSubmatch(memory, -1)
	for _, match := range mibMatches {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			totalGB += val / 1024
		}
	}

	// Match GB values (in case API uses GB instead of GiB)
	gbRe := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*GB`)
	gbMatches := gbRe.FindAllStringSubmatch(memory, -1)
	for _, match := range gbMatches {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			totalGB += val
		}
	}

	return totalGB
}

// gpuCapabilityEntry represents a GPU pattern and its compute capability
type gpuCapabilityEntry struct {
	pattern    string
	capability float64
}

// getGPUCapability returns the compute capability for known GPU types
func getGPUCapability(gpuName string) float64 {
	gpuName = strings.ToUpper(gpuName)

	// Order matters: more specific patterns must come before less specific ones
	// (e.g., "A100" before "A10", "L40S" before "L40")
	capabilities := []gpuCapabilityEntry{
		// NVIDIA Professional (before other RTX patterns)
		{"RTXPRO6000", 12.0},

		// NVIDIA Blackwell
		{"B200", 10.0},
		{"RTX5090", 10.0},

		// NVIDIA Hopper
		{"H100", 9.0},
		{"H200", 9.0},

		// NVIDIA Ada Lovelace (L40S before L40, L4; RTX*Ada before RTX*)
		{"L40S", 8.9},
		{"L40", 8.9},
		{"L4", 8.9},
		{"RTX6000ADA", 8.9},
		{"RTX4000ADA", 8.9},
		{"RTX4090", 8.9},
		{"RTX4080", 8.9},

		// NVIDIA Ampere (A100 before A10G, A10)
		{"A100", 8.0},
		{"A10G", 8.6},
		{"A10", 8.6},
		{"A40", 8.6},
		{"A6000", 8.6},
		{"A5000", 8.6},
		{"A4000", 8.6},
		{"A30", 8.0},
		{"A16", 8.6},
		{"RTX3090", 8.6},
		{"RTX3080", 8.6},

		// NVIDIA Turing
		{"T4", 7.5},
		{"RTX6000", 7.5},
		{"RTX2080", 7.5},

		// NVIDIA Volta
		{"V100", 7.0},

		// NVIDIA Pascal (P100 before P40, P4)
		{"P100", 6.0},
		{"P40", 6.1},
		{"P4", 6.1},

		// NVIDIA Maxwell
		{"M60", 5.2},

		// NVIDIA Kepler
		{"K80", 3.7},

		// Gaudi (Habana) - not CUDA compatible
		{"HL-205", 0},
		{"GAUDI3", 0},
		{"GAUDI2", 0},
		{"GAUDI", 0},
	}

	for _, entry := range capabilities {
		if strings.Contains(gpuName, entry.pattern) {
			return entry.capability
		}
	}
	return 0
}

// processInstances converts raw instance types to GPUInstanceInfo
func processInstances(items []InstanceType) []GPUInstanceInfo {
	var instances []GPUInstanceInfo

	for _, item := range items {
		if len(item.SupportedGPUs) == 0 {
			continue // Skip non-GPU instances
		}

		for _, gpu := range item.SupportedGPUs {
			vramPerGPU := parseMemoryToGB(gpu.Memory)
			// Also check memory_bytes as fallback
			if vramPerGPU == 0 && gpu.MemoryBytes.Value > 0 {
				// Convert based on unit
				if gpu.MemoryBytes.Unit == "MiB" {
					vramPerGPU = float64(gpu.MemoryBytes.Value) / 1024 // MiB to GiB
				} else if gpu.MemoryBytes.Unit == "GiB" {
					vramPerGPU = float64(gpu.MemoryBytes.Value)
				}
			}

			totalVRAM := vramPerGPU * float64(gpu.Count)
			capability := getGPUCapability(gpu.Name)

			price := 0.0
			if item.BasePrice.Amount != "" {
				price, _ = strconv.ParseFloat(item.BasePrice.Amount, 64)
			}

			instances = append(instances, GPUInstanceInfo{
				Type:         item.Type,
				GPUName:      gpu.Name,
				GPUCount:     gpu.Count,
				VRAMPerGPU:   vramPerGPU,
				TotalVRAM:    totalVRAM,
				Capability:   capability,
				VCPUs:        item.VCPU,
				Memory:       item.Memory,
				PricePerHour: price,
				Manufacturer: gpu.Manufacturer,
			})
		}
	}

	return instances
}

// filterInstances applies all filters to the instance list
func filterInstances(instances []GPUInstanceInfo, gpuName string, minVRAM, minTotalVRAM, minCapability float64) []GPUInstanceInfo {
	var filtered []GPUInstanceInfo

	for _, inst := range instances {
		// Filter out non-NVIDIA GPUs (AMD, Intel/Habana, etc.)
		if !strings.Contains(strings.ToUpper(inst.Manufacturer), "NVIDIA") {
			continue
		}

		// Filter by GPU name (case-insensitive partial match)
		if gpuName != "" && !strings.Contains(strings.ToLower(inst.GPUName), strings.ToLower(gpuName)) {
			continue
		}

		// Filter by minimum VRAM per GPU
		if minVRAM > 0 && inst.VRAMPerGPU < minVRAM {
			continue
		}

		// Filter by minimum total VRAM
		if minTotalVRAM > 0 && inst.TotalVRAM < minTotalVRAM {
			continue
		}

		// Filter by minimum GPU capability
		if minCapability > 0 && inst.Capability < minCapability {
			continue
		}

		filtered = append(filtered, inst)
	}

	return filtered
}

// sortInstances sorts the instance list by the specified column
func sortInstances(instances []GPUInstanceInfo, sortBy string, descending bool) {
	sort.Slice(instances, func(i, j int) bool {
		var less bool
		switch strings.ToLower(sortBy) {
		case "price":
			less = instances[i].PricePerHour < instances[j].PricePerHour
		case "gpu-count":
			less = instances[i].GPUCount < instances[j].GPUCount
		case "vram":
			less = instances[i].VRAMPerGPU < instances[j].VRAMPerGPU
		case "total-vram":
			less = instances[i].TotalVRAM < instances[j].TotalVRAM
		case "vcpu":
			less = instances[i].VCPUs < instances[j].VCPUs
		case "type":
			less = instances[i].Type < instances[j].Type
		case "capability":
			less = instances[i].Capability < instances[j].Capability
		default:
			less = instances[i].PricePerHour < instances[j].PricePerHour
		}

		if descending {
			return !less
		}
		return less
	})
}

// getBrevTableOptions returns table styling options
func getBrevTableOptions() table.Options {
	options := table.OptionsDefault
	options.DrawBorder = false
	options.SeparateColumns = false
	options.SeparateRows = false
	options.SeparateHeader = false
	return options
}

// displayGPUTable renders the GPU instances as a table
func displayGPUTable(t *terminal.Terminal, instances []GPUInstanceInfo) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()

	header := table.Row{"TYPE", "GPU", "COUNT", "VRAM/GPU", "TOTAL VRAM", "CAPABILITY", "VCPUs", "$/HR"}
	ta.AppendHeader(header)

	for _, inst := range instances {
		vramStr := fmt.Sprintf("%.0f GB", inst.VRAMPerGPU)
		totalVramStr := fmt.Sprintf("%.0f GB", inst.TotalVRAM)
		capStr := "-"
		if inst.Capability > 0 {
			capStr = fmt.Sprintf("%.1f", inst.Capability)
		}
		priceStr := fmt.Sprintf("$%.2f", inst.PricePerHour)

		row := table.Row{
			inst.Type,
			t.Green(inst.GPUName),
			inst.GPUCount,
			vramStr,
			totalVramStr,
			capStr,
			inst.VCPUs,
			priceStr,
		}
		ta.AppendRow(row)
	}

	ta.Render()
}
