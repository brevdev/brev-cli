// Package gpusearch provides a command to search and filter GPU instance types
package gpusearch

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

var (
	long = `Search and filter GPU instance types available on Brev.

Filter by GPU name, memory, capability, and more. Sort results by different columns.`

	example = `
  # List all GPU instances
  brev gpus

  # Filter by GPU name (case-insensitive, partial match)
  brev gpus --gpu-name a100
  brev gpus -n h100

  # Filter by minimum GPU memory per GPU (in GB)
  brev gpus --gpu-memory 40

  # Filter by minimum total VRAM across all GPUs (in GB)
  brev gpus --total-vram 80

  # Filter by minimum GPU capability (compute capability number)
  brev gpus --capability 8.0

  # Filter by provider
  brev gpus --provider aws
  brev gpus --provider gcp

  # Combine filters
  brev gpus --gpu-name a100 --gpu-memory 40 --provider aws

  # Sort by different columns (default: instance-type)
  brev gpus --sort gpu-name
  brev gpus --sort gpu-memory
  brev gpus --sort total-vram
  brev gpus --sort capability
  brev gpus --sort price
  brev gpus --sort gpu-count

  # Sort in descending order
  brev gpus --sort price --desc

  # Show only available instances
  brev gpus --available
`
)

// GPUSearchStore defines the interface for fetching GPU instance types
type GPUSearchStore interface {
	GetInstanceTypes() ([]entity.GPUInstanceType, error)
}

// NewCmdGPUSearch creates the gpus command
func NewCmdGPUSearch(t *terminal.Terminal, store GPUSearchStore) *cobra.Command {
	var gpuName string
	var gpuMemory float64
	var totalVRAM float64
	var capability float64
	var provider string
	var sortBy string
	var descending bool
	var availableOnly bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "gpus",
		Aliases:               []string{"gpu-search", "gpu"},
		DisableFlagsInUseLine: true,
		Short:                 "Search and filter GPU instance types",
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunGPUSearch(t, store, GPUSearchOptions{
				GPUName:       gpuName,
				GPUMemory:     gpuMemory,
				TotalVRAM:     totalVRAM,
				Capability:    capability,
				Provider:      provider,
				SortBy:        sortBy,
				Descending:    descending,
				AvailableOnly: availableOnly,
			})
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&gpuName, "gpu-name", "n", "", "Filter by GPU name (case-insensitive, partial match)")
	cmd.Flags().Float64VarP(&gpuMemory, "gpu-memory", "m", 0, "Filter by minimum GPU memory per GPU (in GB)")
	cmd.Flags().Float64VarP(&totalVRAM, "total-vram", "v", 0, "Filter by minimum total VRAM across all GPUs (in GB)")
	cmd.Flags().Float64VarP(&capability, "capability", "c", 0, "Filter by minimum GPU capability (compute capability number)")
	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Filter by cloud provider (e.g., aws, gcp)")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "instance-type", "Sort by column: instance-type, gpu-name, gpu-memory, total-vram, capability, price, gpu-count")
	cmd.Flags().BoolVarP(&descending, "desc", "d", false, "Sort in descending order")
	cmd.Flags().BoolVarP(&availableOnly, "available", "a", false, "Show only available instances")

	return cmd
}

// GPUSearchOptions contains the filter and sort options
type GPUSearchOptions struct {
	GPUName       string
	GPUMemory     float64
	TotalVRAM     float64
	Capability    float64
	Provider      string
	SortBy        string
	Descending    bool
	AvailableOnly bool
}

// RunGPUSearch executes the GPU search with the given options
func RunGPUSearch(t *terminal.Terminal, store GPUSearchStore, opts GPUSearchOptions) error {
	instances, err := store.GetInstanceTypes()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if len(instances) == 0 {
		t.Vprint(t.Yellow("No GPU instance types found."))
		return nil
	}

	// Apply filters
	filtered := filterInstances(instances, opts)

	if len(filtered) == 0 {
		t.Vprint(t.Yellow("No GPU instances match the specified filters."))
		return nil
	}

	// Sort results
	sortInstances(filtered, opts.SortBy, opts.Descending)

	// Display results
	displayInstancesTable(t, filtered)

	t.Vprintf("\nFound %d GPU instance type(s)\n", len(filtered))

	return nil
}

// filterInstances applies all filters to the instance list
func filterInstances(instances []entity.GPUInstanceType, opts GPUSearchOptions) []entity.GPUInstanceType {
	var filtered []entity.GPUInstanceType

	for _, inst := range instances {
		// Filter by availability
		if opts.AvailableOnly && !inst.Available {
			continue
		}

		// Filter by GPU name (case-insensitive, partial match)
		if opts.GPUName != "" {
			if !strings.Contains(strings.ToLower(inst.GPUName), strings.ToLower(opts.GPUName)) {
				continue
			}
		}

		// Filter by minimum GPU memory per GPU
		if opts.GPUMemory > 0 && inst.GPUMemoryGB < opts.GPUMemory {
			continue
		}

		// Filter by minimum total VRAM
		if opts.TotalVRAM > 0 && inst.TotalGPUMemoryGB < opts.TotalVRAM {
			continue
		}

		// Filter by minimum GPU capability
		if opts.Capability > 0 && inst.GPUCapability < opts.Capability {
			continue
		}

		// Filter by provider (case-insensitive)
		if opts.Provider != "" {
			if !strings.EqualFold(inst.Provider, opts.Provider) {
				continue
			}
		}

		filtered = append(filtered, inst)
	}

	return filtered
}

// sortInstances sorts the instances by the specified column
func sortInstances(instances []entity.GPUInstanceType, sortBy string, descending bool) {
	sort.Slice(instances, func(i, j int) bool {
		var less bool

		switch strings.ToLower(sortBy) {
		case "gpu-name", "gpuname", "name":
			less = strings.ToLower(instances[i].GPUName) < strings.ToLower(instances[j].GPUName)
		case "gpu-memory", "gpumemory", "memory":
			less = instances[i].GPUMemoryGB < instances[j].GPUMemoryGB
		case "total-vram", "totalvram", "vram":
			less = instances[i].TotalGPUMemoryGB < instances[j].TotalGPUMemoryGB
		case "capability", "cap":
			less = instances[i].GPUCapability < instances[j].GPUCapability
		case "price", "cost":
			less = instances[i].PricePerHour < instances[j].PricePerHour
		case "gpu-count", "gpucount", "count", "gpus":
			less = instances[i].GPUCount < instances[j].GPUCount
		case "provider":
			less = strings.ToLower(instances[i].Provider) < strings.ToLower(instances[j].Provider)
		case "vcpus", "cpu":
			less = instances[i].VCPUs < instances[j].VCPUs
		default: // instance-type
			less = strings.ToLower(instances[i].InstanceType) < strings.ToLower(instances[j].InstanceType)
		}

		if descending {
			return !less
		}
		return less
	})
}

// displayInstancesTable renders the GPU instances in a table format
func displayInstancesTable(t *terminal.Terminal, instances []entity.GPUInstanceType) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()

	header := table.Row{"Instance Type", "GPU", "GPU Mem", "GPUs", "Total VRAM", "Capability", "vCPUs", "RAM", "$/hr", "Provider", "Available"}
	ta.AppendHeader(header)

	for _, inst := range instances {
		availableStr := formatAvailable(t, inst.Available)
		row := table.Row{
			inst.InstanceType,
			inst.GPUName,
			formatMemory(inst.GPUMemoryGB),
			inst.GPUCount,
			formatMemory(inst.TotalGPUMemoryGB),
			formatCapability(inst.GPUCapability),
			inst.VCPUs,
			formatMemory(inst.MemoryGB),
			formatPrice(inst.PricePerHour),
			inst.Provider,
			availableStr,
		}
		ta.AppendRow(row)
	}

	ta.Render()
}

func getBrevTableOptions() table.Options {
	options := table.OptionsDefault
	options.DrawBorder = false
	options.SeparateColumns = false
	options.SeparateRows = false
	options.SeparateHeader = false
	return options
}

func formatMemory(gb float64) string {
	if gb == 0 {
		return "-"
	}
	if gb == float64(int(gb)) {
		return fmt.Sprintf("%dGB", int(gb))
	}
	return fmt.Sprintf("%.1fGB", gb)
}

func formatCapability(cap float64) string {
	if cap == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", cap)
}

func formatPrice(price float64) string {
	if price == 0 {
		return "-"
	}
	return fmt.Sprintf("$%.2f", price)
}

func formatAvailable(t *terminal.Terminal, available bool) string {
	if available {
		return t.Green("Yes")
	}
	return t.Red("No")
}
