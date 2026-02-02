// Package gpusearch provides a command to search and filter GPU instance types
package gpusearch

import (
	"encoding/json"
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

// Storage represents a storage configuration within an instance type
type Storage struct {
	Count        int         `json:"count"`
	Size         string      `json:"size"`
	Type         string      `json:"type"`
	MinSize      string      `json:"min_size"`
	MaxSize      string      `json:"max_size"`
	SizeBytes    MemoryBytes `json:"size_bytes"`
	PricePerGBHr BasePrice   `json:"price_per_gb_hr"` // Uses BasePrice since API returns {currency, amount}
}

// WorkspaceGroup represents a workspace group that can run an instance type
type WorkspaceGroup struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	PlatformType string `json:"platformType"`
}

// InstanceType represents an instance type from the API
type InstanceType struct {
	Type                   string           `json:"type"`
	SupportedGPUs          []GPU            `json:"supported_gpus"`
	SupportedStorage       []Storage        `json:"supported_storage"`
	Memory                 string           `json:"memory"`
	VCPU                   int              `json:"vcpu"`
	BasePrice              BasePrice        `json:"base_price"`
	Location               string           `json:"location"`
	SubLocation            string           `json:"sub_location"`
	AvailableLocations     []string         `json:"available_locations"`
	Provider               string           `json:"provider"`
	WorkspaceGroups        []WorkspaceGroup `json:"workspace_groups"`
	EstimatedDeployTime    string           `json:"estimated_deploy_time"`
	Stoppable              bool             `json:"stoppable"`
	Rebootable             bool             `json:"rebootable"`
	CanModifyFirewallRules bool             `json:"can_modify_firewall_rules"`
}

// InstanceTypesResponse represents the API response
type InstanceTypesResponse struct {
	Items []InstanceType `json:"items"`
}

// AllInstanceTypesResponse represents the authenticated API response with workspace groups
type AllInstanceTypesResponse struct {
	AllInstanceTypes []InstanceType `json:"allInstanceTypes"`
}

// GetWorkspaceGroupID returns the workspace group ID for an instance type, or empty string if not found
func (r *AllInstanceTypesResponse) GetWorkspaceGroupID(instanceType string) string {
	for _, it := range r.AllInstanceTypes {
		if it.Type == instanceType {
			if len(it.WorkspaceGroups) > 0 {
				return it.WorkspaceGroups[0].ID
			}
		}
	}
	return ""
}

// GPUSearchStore defines the interface for fetching instance types
type GPUSearchStore interface {
	GetInstanceTypes() (*InstanceTypesResponse, error)
}

var (
	long = `Search and filter GPU instance types available on Brev.

Filter instances by GPU name, provider, VRAM, total VRAM, GPU compute capability, disk size, and boot time.
Sort results by various columns to find the best instance for your needs.

Features column shows instance capabilities:
  S = Stoppable (can stop and restart without losing data)
  R = Rebootable (can reboot the instance)
  P = Flex Ports (can modify firewall/port rules)`

	example = `
  # List all GPU instances
  brev search

  # Filter by GPU name (case-insensitive, partial match)
  brev search --gpu-name A100
  brev search --gpu-name "L40S"

  # Filter by provider/cloud (case-insensitive, partial match)
  brev search --provider aws
  brev search --provider gcp

  # Filter by minimum VRAM per GPU (in GB)
  brev search --min-vram 24

  # Filter by minimum total VRAM (in GB)
  brev search --min-total-vram 80

  # Filter by minimum GPU compute capability
  brev search --min-capability 8.0

  # Filter by minimum disk size (in GB)
  brev search --min-disk 500

  # Filter by maximum boot time (in minutes)
  brev search --max-boot-time 5

  # Filter by instance features
  brev search --stoppable              # Only show instances that can be stopped/restarted
  brev search --rebootable             # Only show instances that can be rebooted
  brev search --flex-ports             # Only show instances with configurable firewall rules

  # Sort by different columns (price, gpu-count, vram, total-vram, vcpu, provider, disk, boot-time)
  brev search --sort price
  brev search --sort boot-time
  brev search --sort disk --desc

  # Combine filters
  brev search --gpu-name A100 --min-vram 40 --sort price
  brev search --gpu-name H100 --max-boot-time 3 --sort price
  brev search --stoppable --min-total-vram 40 --sort price
`
)

// NewCmdGPUSearch creates the search command
func NewCmdGPUSearch(t *terminal.Terminal, store GPUSearchStore) *cobra.Command {
	var gpuName string
	var provider string
	var minVRAM float64
	var minTotalVRAM float64
	var minCapability float64
	var minDisk float64
	var maxBootTime int
	var stoppable bool
	var rebootable bool
	var flexPorts bool
	var sortBy string
	var descending bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "search",
		Aliases:               []string{"gpu-search", "gpu", "gpus", "gpu-list"},
		DisableFlagsInUseLine: true,
		Short:                 "Search and filter GPU instance types",
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunGPUSearch(t, store, gpuName, provider, minVRAM, minTotalVRAM, minCapability, minDisk, maxBootTime, stoppable, rebootable, flexPorts, sortBy, descending, jsonOutput)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&gpuName, "gpu-name", "g", "", "Filter by GPU name (case-insensitive, partial match)")
	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Filter by provider/cloud (case-insensitive, partial match)")
	cmd.Flags().Float64VarP(&minVRAM, "min-vram", "v", 0, "Minimum VRAM per GPU in GB")
	cmd.Flags().Float64VarP(&minTotalVRAM, "min-total-vram", "t", 0, "Minimum total VRAM (GPU count * VRAM) in GB")
	cmd.Flags().Float64VarP(&minCapability, "min-capability", "c", 0, "Minimum GPU compute capability (e.g., 8.0 for Ampere)")
	cmd.Flags().Float64Var(&minDisk, "min-disk", 0, "Minimum disk size in GB")
	cmd.Flags().IntVar(&maxBootTime, "max-boot-time", 0, "Maximum boot time in minutes")
	cmd.Flags().BoolVar(&stoppable, "stoppable", false, "Only show instances that can be stopped and restarted")
	cmd.Flags().BoolVar(&rebootable, "rebootable", false, "Only show instances that can be rebooted")
	cmd.Flags().BoolVar(&flexPorts, "flex-ports", false, "Only show instances with configurable firewall/port rules")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "price", "Sort by: price, gpu-count, vram, total-vram, vcpu, type, provider, disk, boot-time")
	cmd.Flags().BoolVarP(&descending, "desc", "d", false, "Sort in descending order")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")

	return cmd
}

// GPUInstanceInfo holds processed GPU instance information for display
type GPUInstanceInfo struct {
	Type           string  `json:"type"`
	Cloud          string  `json:"cloud"`    // Underlying cloud (e.g., hyperstack, aws, gcp)
	Provider       string  `json:"provider"` // Provider/aggregator (e.g., shadeform, aws, gcp)
	GPUName        string  `json:"gpu_name"`
	GPUCount       int     `json:"gpu_count"`
	VRAMPerGPU     float64 `json:"vram_per_gpu_gb"`
	TotalVRAM      float64 `json:"total_vram_gb"`
	Capability     float64 `json:"capability"`
	VCPUs          int     `json:"vcpus"`
	Memory         string  `json:"memory"`
	DiskMin        float64 `json:"disk_min_gb"`
	DiskMax        float64 `json:"disk_max_gb"`
	DiskPricePerMo float64 `json:"disk_price_per_gb_mo,omitempty"` // $/GB/month for flexible storage
	BootTime       int     `json:"boot_time_seconds"`
	Stoppable      bool    `json:"stoppable"`
	Rebootable     bool    `json:"rebootable"`
	FlexPorts      bool    `json:"flex_ports"`
	TargetDisk     float64 `json:"target_disk_gb,omitempty"`
	PricePerHour   float64 `json:"price_per_hour"`
	Manufacturer   string  `json:"-"` // exclude from JSON output
}

// isStdoutPiped returns true if stdout is being piped (not a terminal)
func isStdoutPiped() bool {
	stat, _ := os.Stdout.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// RunGPUSearch executes the GPU search with filters and sorting
func RunGPUSearch(t *terminal.Terminal, store GPUSearchStore, gpuName, provider string, minVRAM, minTotalVRAM, minCapability, minDisk float64, maxBootTime int, stoppable, rebootable, flexPorts bool, sortBy string, descending, jsonOutput bool) error {
	// Detect if stdout is piped (for plain table output)
	piped := isStdoutPiped()

	response, err := store.GetInstanceTypes()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if response == nil || len(response.Items) == 0 {
		if jsonOutput {
			fmt.Println("[]")
			return nil
		}
		if piped {
			return nil
		}
		t.Vprint(t.Yellow("No instance types found"))
		return nil
	}

	// Process and filter instances
	instances := ProcessInstances(response.Items)

	// Apply filters
	filtered := FilterInstances(instances, gpuName, provider, minVRAM, minTotalVRAM, minCapability, minDisk, maxBootTime, stoppable, rebootable, flexPorts)

	if len(filtered) == 0 {
		if jsonOutput {
			fmt.Println("[]")
			return nil
		}
		if piped {
			return nil
		}
		t.Vprint(t.Yellow("No GPU instances match the specified filters"))
		return nil
	}

	// Set target disk for each instance
	// For flexible storage, use minDisk if specified and within range
	for i := range filtered {
		inst := &filtered[i]
		if inst.DiskMin != inst.DiskMax && minDisk > 0 && minDisk >= inst.DiskMin && minDisk <= inst.DiskMax {
			inst.TargetDisk = minDisk
		} else {
			inst.TargetDisk = inst.DiskMin
		}
	}

	// Sort instances
	SortInstances(filtered, sortBy, descending)

	// Display results
	if jsonOutput {
		return displayGPUJSON(filtered)
	}

	// Plain table when piped - enables: brev search | grep H100 | brev create
	if piped {
		displayGPUTablePlain(filtered)
		return nil
	}

	displayGPUTable(t, filtered)
	t.Vprintf("\n%s\n", t.Green(fmt.Sprintf("Found %d GPU instance types", len(filtered))))

	return nil
}

// displayGPUJSON outputs the GPU instances as JSON
func displayGPUJSON(instances []GPUInstanceInfo) error {
	output, err := json.MarshalIndent(instances, "", "  ")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println(string(output))
	return nil
}

// unitMultipliers maps size units to their GB equivalent
var unitMultipliers = map[string]float64{
	"TiB": 1024,
	"TB":  1000,
	"GiB": 1,
	"GB":  1,
	"MiB": 1.0 / 1024,
	"MB":  1.0 / 1000,
}

// parseToGB converts size/memory strings like "22GiB360MiB", "16TiB", "2TiB768GiB" to GB
func parseToGB(s string) float64 {
	var totalGB float64
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(TiB|TB|GiB|GB|MiB|MB)`)
	for _, match := range re.FindAllStringSubmatch(s, -1) {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			totalGB += val * unitMultipliers[match[2]]
		}
	}
	return totalGB
}

// parseMemoryToGB converts memory string like "22GiB360MiB" or "40GiB" to GB
func parseMemoryToGB(memory string) float64 {
	return parseToGB(memory)
}

// parseSizeToGB parses size strings like "16TiB", "10GiB", "2TiB768GiB" to GB
func parseSizeToGB(size string) float64 {
	return parseToGB(size)
}

// parseDurationToSeconds parses Go duration strings like "7m0s", "1m30s" to seconds
func parseDurationToSeconds(duration string) int {
	var totalSeconds int

	// Match hours
	hRe := regexp.MustCompile(`(\d+)h`)
	if match := hRe.FindStringSubmatch(duration); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			totalSeconds += val * 3600
		}
	}

	// Match minutes
	mRe := regexp.MustCompile(`(\d+)m`)
	if match := mRe.FindStringSubmatch(duration); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			totalSeconds += val * 60
		}
	}

	// Match seconds
	sRe := regexp.MustCompile(`(\d+)s`)
	if match := sRe.FindStringSubmatch(duration); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			totalSeconds += val
		}
	}

	return totalSeconds
}

// extractDiskInfo extracts min/max disk size and price from storage configuration
// Returns (minGB, maxGB, pricePerGBMonth). For fixed-size storage, min==max and price is 0.
func extractDiskInfo(storage []Storage) (float64, float64, float64) {
	if len(storage) == 0 {
		return 0, 0, 0
	}

	// Use the first storage entry
	s := storage[0]

	// Convert price per GB per hour to per month (730 hours average)
	var pricePerGBMonth float64
	if s.PricePerGBHr.Amount != "" {
		pricePerHr, _ := strconv.ParseFloat(s.PricePerGBHr.Amount, 64)
		if pricePerHr > 0 {
			pricePerGBMonth = pricePerHr * 730
		}
	}

	// Check if it's flexible storage (has min_size and max_size)
	if s.MinSize != "" && s.MaxSize != "" {
		minGB := parseSizeToGB(s.MinSize)
		maxGB := parseSizeToGB(s.MaxSize)
		return minGB, maxGB, pricePerGBMonth
	}

	// Fixed storage - use size or size_bytes
	var sizeGB float64
	if s.Size != "" && s.Size != "0B" {
		sizeGB = parseSizeToGB(s.Size)
	}

	// Fallback to size_bytes
	if sizeGB == 0 && s.SizeBytes.Value > 0 {
		switch s.SizeBytes.Unit {
		case "GiB", "GB":
			sizeGB = float64(s.SizeBytes.Value)
		case "MiB", "MB":
			sizeGB = float64(s.SizeBytes.Value) / 1024
		case "TiB", "TB":
			sizeGB = float64(s.SizeBytes.Value) * 1024
		}
	}

	// Fixed storage doesn't have separate pricing - it's included in base price
	return sizeGB, sizeGB, 0
}

// extractCloud extracts the underlying cloud from the instance type and provider
// For aggregators like shadeform, the cloud is in the type name prefix (e.g., "hyperstack_H100" -> "hyperstack")
// For direct providers like aws/gcp, the provider IS the cloud
func extractCloud(instanceType, provider string) string {
	// Direct cloud providers - provider is the cloud
	directProviders := map[string]bool{
		"aws": true, "gcp": true, "azure": true, "oci": true,
		"nebius": true, "crusoe": true, "lambda-labs": true, "launchpad": true,
	}

	if directProviders[strings.ToLower(provider)] {
		return provider
	}

	// For aggregators, try to extract cloud from type name prefix
	// Pattern: cloudname_GPUtype (e.g., "hyperstack_H100", "cudo_A40", "latitude_H100")
	if idx := strings.Index(instanceType, "_"); idx > 0 {
		return instanceType[:idx]
	}

	// Fallback to provider
	return provider
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
		{"B300", 10.3},
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

// ProcessInstances converts raw instance types to GPUInstanceInfo
func ProcessInstances(items []InstanceType) []GPUInstanceInfo {
	var instances []GPUInstanceInfo

	for _, item := range items {
		if len(item.SupportedGPUs) == 0 {
			continue // Skip non-GPU instances
		}

		// Extract disk size and price info from first storage entry
		diskMin, diskMax, diskPricePerMo := extractDiskInfo(item.SupportedStorage)

		// Extract boot time
		bootTime := parseDurationToSeconds(item.EstimatedDeployTime)

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
				Type:           item.Type,
				Cloud:          extractCloud(item.Type, item.Provider),
				Provider:       item.Provider,
				GPUName:        gpu.Name,
				GPUCount:       gpu.Count,
				VRAMPerGPU:     vramPerGPU,
				TotalVRAM:      totalVRAM,
				Capability:     capability,
				VCPUs:          item.VCPU,
				Memory:         item.Memory,
				DiskMin:        diskMin,
				DiskMax:        diskMax,
				DiskPricePerMo: diskPricePerMo,
				BootTime:       bootTime,
				Stoppable:      item.Stoppable,
				Rebootable:     item.Rebootable,
				FlexPorts:      item.CanModifyFirewallRules,
				PricePerHour:   price,
				Manufacturer:   gpu.Manufacturer,
			})
		}
	}

	return instances
}

// FilterOptions holds all filter criteria for GPU instances
type FilterOptions struct {
	GPUName       string
	Provider      string
	MinVRAM       float64
	MinTotalVRAM  float64
	MinCapability float64
	MinDisk       float64
	MaxBootTime   int // in minutes
	Stoppable     bool
	Rebootable    bool
	FlexPorts     bool
}

// matchesStringFilters checks GPU name and provider filters
func (f *FilterOptions) matchesStringFilters(inst GPUInstanceInfo) bool {
	// Filter out non-NVIDIA GPUs (AMD, Intel/Habana, etc.)
	if !strings.Contains(strings.ToUpper(inst.Manufacturer), "NVIDIA") {
		return false
	}
	// Filter by GPU name (case-insensitive partial match)
	if f.GPUName != "" && !strings.Contains(strings.ToLower(inst.GPUName), strings.ToLower(f.GPUName)) {
		return false
	}
	// Filter by provider (case-insensitive partial match)
	if f.Provider != "" && !strings.Contains(strings.ToLower(inst.Provider), strings.ToLower(f.Provider)) {
		return false
	}
	return true
}

// matchesNumericFilters checks VRAM, capability, disk, and boot time filters
func (f *FilterOptions) matchesNumericFilters(inst GPUInstanceInfo) bool {
	if f.MinVRAM > 0 && inst.VRAMPerGPU < f.MinVRAM {
		return false
	}
	if f.MinTotalVRAM > 0 && inst.TotalVRAM < f.MinTotalVRAM {
		return false
	}
	if f.MinCapability > 0 && inst.Capability < f.MinCapability {
		return false
	}
	if f.MinDisk > 0 && inst.DiskMax < f.MinDisk {
		return false
	}
	// Exclude instances with unknown boot time (0) when filter is specified
	if f.MaxBootTime > 0 && (inst.BootTime == 0 || inst.BootTime > f.MaxBootTime*60) {
		return false
	}
	return true
}

// matchesFeatureFilters checks stoppable, rebootable, and flex ports filters
func (f *FilterOptions) matchesFeatureFilters(inst GPUInstanceInfo) bool {
	if f.Stoppable && !inst.Stoppable {
		return false
	}
	if f.Rebootable && !inst.Rebootable {
		return false
	}
	if f.FlexPorts && !inst.FlexPorts {
		return false
	}
	return true
}

// matchesFilter checks if an instance matches all filter criteria
func (f *FilterOptions) matchesFilter(inst GPUInstanceInfo) bool {
	return f.matchesStringFilters(inst) &&
		f.matchesNumericFilters(inst) &&
		f.matchesFeatureFilters(inst)
}

// FilterInstances applies all filters to the instance list
func FilterInstances(instances []GPUInstanceInfo, gpuName, provider string, minVRAM, minTotalVRAM, minCapability, minDisk float64, maxBootTime int, stoppable, rebootable, flexPorts bool) []GPUInstanceInfo {
	opts := &FilterOptions{
		GPUName:       gpuName,
		Provider:      provider,
		MinVRAM:       minVRAM,
		MinTotalVRAM:  minTotalVRAM,
		MinCapability: minCapability,
		MinDisk:       minDisk,
		MaxBootTime:   maxBootTime,
		Stoppable:     stoppable,
		Rebootable:    rebootable,
		FlexPorts:     flexPorts,
	}

	var filtered []GPUInstanceInfo
	for _, inst := range instances {
		if opts.matchesFilter(inst) {
			filtered = append(filtered, inst)
		}
	}
	return filtered
}

// SortInstances sorts the instance list by the specified column
func SortInstances(instances []GPUInstanceInfo, sortBy string, descending bool) {
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
		case "provider":
			less = instances[i].Provider < instances[j].Provider
		case "disk":
			less = instances[i].DiskMax < instances[j].DiskMax
		case "boot-time":
			// Instances with no boot time (0) should always appear last
			switch {
			case instances[i].BootTime == 0 && instances[j].BootTime == 0:
				return false // both unknown, equal
			case instances[i].BootTime == 0:
				return false // i unknown goes after j
			case instances[j].BootTime == 0:
				return true // j unknown goes after i
			}
			less = instances[i].BootTime < instances[j].BootTime
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

// formatDiskSize formats the disk size for display
func formatDiskSize(minGB, maxGB float64) string {
	if minGB == 0 && maxGB == 0 {
		return "-"
	}

	formatSize := func(gb float64) string {
		if gb >= 1000 {
			return fmt.Sprintf("%.0fTB", gb/1000)
		}
		return fmt.Sprintf("%.0fGB", gb)
	}

	if minGB == maxGB {
		// Fixed size
		return formatSize(minGB)
	}
	// Range
	return fmt.Sprintf("%s-%s", formatSize(minGB), formatSize(maxGB))
}

// formatBootTime formats boot time in seconds to a human-readable string
func formatBootTime(seconds int) string {
	if seconds == 0 {
		return "-"
	}
	minutes := seconds / 60
	secs := seconds % 60
	if secs == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm%ds", minutes, secs)
}

// formatFeatures formats feature flags as abbreviated string
// S=stoppable, R=rebootable, P=flex ports (can modify firewall)
func formatFeatures(stoppable, rebootable, flexPorts bool) string {
	var features []string
	if stoppable {
		features = append(features, "S")
	}
	if rebootable {
		features = append(features, "R")
	}
	if flexPorts {
		features = append(features, "P")
	}
	if len(features) == 0 {
		return "-"
	}
	return strings.Join(features, "")
}

// displayGPUTable renders the GPU instances as a table
func displayGPUTable(t *terminal.Terminal, instances []GPUInstanceInfo) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()

	header := table.Row{"TYPE", "PROVIDER", "GPU", "COUNT", "VRAM/GPU", "TOTAL VRAM", "CAPABILITY", "DISK", "$/GB/MO", "BOOT", "FEATURES", "VCPUs", "$/HR"}
	ta.AppendHeader(header)

	for _, inst := range instances {
		vramStr := fmt.Sprintf("%.0f GB", inst.VRAMPerGPU)
		totalVramStr := fmt.Sprintf("%.0f GB", inst.TotalVRAM)
		capStr := "-"
		if inst.Capability > 0 {
			capStr = fmt.Sprintf("%.1f", inst.Capability)
		}
		diskStr := formatDiskSize(inst.DiskMin, inst.DiskMax)
		diskPriceStr := "-"
		if inst.DiskPricePerMo > 0 {
			diskPriceStr = fmt.Sprintf("$%.2f", inst.DiskPricePerMo)
		}
		bootStr := formatBootTime(inst.BootTime)
		featuresStr := formatFeatures(inst.Stoppable, inst.Rebootable, inst.FlexPorts)
		priceStr := fmt.Sprintf("$%.2f", inst.PricePerHour)

		// Format cloud:provider - only show both if different
		providerStr := inst.Provider
		if inst.Cloud != "" && inst.Cloud != inst.Provider {
			providerStr = fmt.Sprintf("%s:%s", inst.Cloud, inst.Provider)
		}

		row := table.Row{
			inst.Type,
			providerStr,
			t.Green(inst.GPUName),
			inst.GPUCount,
			vramStr,
			totalVramStr,
			capStr,
			diskStr,
			diskPriceStr,
			bootStr,
			featuresStr,
			inst.VCPUs,
			priceStr,
		}
		ta.AppendRow(row)
	}

	ta.Render()
}

// displayGPUTablePlain renders the GPU instances as a plain table without colors for piping
// Includes TARGET_DISK column for passing disk size to brev create
// Enables: brev search --min-disk 500 | grep H100 | brev create
func displayGPUTablePlain(instances []GPUInstanceInfo) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()

	header := table.Row{"TYPE", "TARGET_DISK", "PROVIDER", "GPU", "COUNT", "VRAM/GPU", "TOTAL_VRAM", "CAPABILITY", "DISK", "$/GB/MO", "BOOT", "FEATURES", "VCPUs", "$/HR"}
	ta.AppendHeader(header)

	for _, inst := range instances {
		vramStr := fmt.Sprintf("%.0f", inst.VRAMPerGPU)
		totalVramStr := fmt.Sprintf("%.0f", inst.TotalVRAM)
		capStr := "-"
		if inst.Capability > 0 {
			capStr = fmt.Sprintf("%.1f", inst.Capability)
		}
		diskStr := formatDiskSize(inst.DiskMin, inst.DiskMax)
		diskPriceStr := "-"
		if inst.DiskPricePerMo > 0 {
			diskPriceStr = fmt.Sprintf("$%.2f", inst.DiskPricePerMo)
		}
		bootStr := formatBootTime(inst.BootTime)
		featuresStr := formatFeatures(inst.Stoppable, inst.Rebootable, inst.FlexPorts)
		priceStr := fmt.Sprintf("$%.2f", inst.PricePerHour)

		// Format target disk as integer GB
		targetDiskStr := fmt.Sprintf("%.0f", inst.TargetDisk)

		// Format cloud:provider - only show both if different
		providerStr := inst.Provider
		if inst.Cloud != "" && inst.Cloud != inst.Provider {
			providerStr = fmt.Sprintf("%s:%s", inst.Cloud, inst.Provider)
		}

		row := table.Row{
			inst.Type,
			targetDiskStr,
			providerStr,
			inst.GPUName,
			inst.GPUCount,
			vramStr,
			totalVramStr,
			capStr,
			diskStr,
			diskPriceStr,
			bootStr,
			featuresStr,
			inst.VCPUs,
			priceStr,
		}
		ta.AppendRow(row)
	}

	ta.Render()
}
