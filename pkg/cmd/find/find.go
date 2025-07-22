package find

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

type FindStore interface {
	GetInstanceTypes() (*store.InstanceTypesResponse, error)
}

type FilterOptions struct {
	GPU            string
	MinGPUCount    int
	MinDisk        string
	Provider       string
	MinNodeVRAM    int
	MaxHourlyPrice float64
	MinGPUVRAM     int
	Capabilities   []string
	MinRAM         int
	MinCPU         int
}

func NewCmdFind(t *terminal.Terminal, findStore FindStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find",
		Short: "Find resources",
		Long:  "Find and filter various Brev resources",
	}

	cmd.AddCommand(NewCmdFindInstanceType(t, findStore))
	return cmd
}

func NewCmdFindInstanceType(t *terminal.Terminal, findStore FindStore) *cobra.Command {
	var opts FilterOptions

	cmd := &cobra.Command{
		Use:   "instance-type",
		Short: "Find instance types matching criteria",
		Long:  "Find and filter instance types based on GPU, memory, price, and other criteria",
		Example: `  brev find instance-type --gpu a100 --min-gpu-count 2 --min-disk 500GB
  brev find instance-type --gpu a100 --provider aws
  brev find instance-type --min-node-vram 40 --max-hourly-price 2.5
  brev find instance-type --min-gpu-vram 40 --capabilities stoppable,rebootable
  brev find instance-type --min-ram 8 --min-cpu 2`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
		Args: cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFindInstanceType(t, findStore, opts)
		},
	}

	cmd.Flags().StringVar(&opts.GPU, "gpu", "", "GPU type to filter by (e.g., a100, t4)")
	cmd.Flags().IntVar(&opts.MinGPUCount, "min-gpu-count", 0, "Minimum number of GPUs")
	cmd.Flags().StringVar(&opts.MinDisk, "min-disk", "", "Minimum disk size (e.g., 500GB)")
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "Cloud provider (e.g., aws, gcp)")
	cmd.Flags().IntVar(&opts.MinNodeVRAM, "min-node-vram", 0, "Minimum node VRAM (total across all GPUs) in GB")
	cmd.Flags().Float64Var(&opts.MaxHourlyPrice, "max-hourly-price", 0, "Maximum hourly price in USD")
	cmd.Flags().IntVar(&opts.MinGPUVRAM, "min-gpu-vram", 0, "Minimum VRAM per GPU in GB")
	cmd.Flags().StringSliceVar(&opts.Capabilities, "capabilities", []string{}, "Required capabilities (comma-separated: stoppable,rebootable)")
	cmd.Flags().IntVar(&opts.MinRAM, "min-ram", 0, "Minimum RAM in GB")
	cmd.Flags().IntVar(&opts.MinCPU, "min-cpu", 0, "Minimum number of CPU cores")

	return cmd
}

func runFindInstanceType(t *terminal.Terminal, findStore FindStore, opts FilterOptions) error {
	response, err := findStore.GetInstanceTypes()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	filtered := filterInstanceTypes(response.Items, opts)
	sortInstanceTypesByPrice(filtered)

	if len(filtered) == 0 {
		t.Vprint(t.Yellow("No instance types found matching the specified criteria."))
		return nil
	}

	displayInstanceTypesTable(t, filtered)
	t.Vprint(t.Green(fmt.Sprintf("\nFound %d instance types matching your criteria, sorted by price.\n", len(filtered))))

	return nil
}

func filterInstanceTypes(instances []store.InstanceType, opts FilterOptions) []store.InstanceType {
	var filtered []store.InstanceType

	for _, instance := range instances {
		if matchesFilters(instance, opts) {
			filtered = append(filtered, instance)
		}
	}

	return filtered
}

func matchesFilters(instance store.InstanceType, opts FilterOptions) bool {
	if opts.GPU != "" && !hasGPU(instance, opts.GPU) {
		return false
	}

	if opts.MinGPUCount > 0 && !hasMinGPUCount(instance, opts.MinGPUCount) {
		return false
	}

	if opts.Provider != "" && !strings.EqualFold(instance.Provider, opts.Provider) {
		return false
	}

	if opts.MinNodeVRAM > 0 && !hasMinNodeVRAM(instance, opts.MinNodeVRAM) {
		return false
	}

	if opts.MaxHourlyPrice > 0 && !belowMaxPrice(instance, opts.MaxHourlyPrice) {
		return false
	}

	if opts.MinGPUVRAM > 0 && !hasMinGPUVRAM(instance, opts.MinGPUVRAM) {
		return false
	}

	if len(opts.Capabilities) > 0 && !hasCapabilities(instance, opts.Capabilities) {
		return false
	}

	if opts.MinRAM > 0 && !hasMinRAM(instance, opts.MinRAM) {
		return false
	}

	if opts.MinCPU > 0 && !hasMinCPU(instance, opts.MinCPU) {
		return false
	}

	return true
}

func hasGPU(instance store.InstanceType, gpuType string) bool {
	for _, gpu := range instance.SupportedGPUs {
		if strings.Contains(strings.ToLower(gpu.Name), strings.ToLower(gpuType)) {
			return true
		}
	}
	return false
}

func hasMinGPUCount(instance store.InstanceType, minCount int) bool {
	totalGPUs := 0
	for _, gpu := range instance.SupportedGPUs {
		totalGPUs += gpu.Count
	}
	return totalGPUs >= minCount
}

func hasMinNodeVRAM(instance store.InstanceType, minVRAM int) bool {
	totalVRAM := 0
	for _, gpu := range instance.SupportedGPUs {
		vram := parseMemoryToGB(gpu.Memory)
		totalVRAM += vram * gpu.Count
	}
	return totalVRAM >= minVRAM
}

func hasMinGPUVRAM(instance store.InstanceType, minVRAM int) bool {
	for _, gpu := range instance.SupportedGPUs {
		vram := parseMemoryToGB(gpu.Memory)
		if vram >= minVRAM {
			return true
		}
	}
	return false
}

func belowMaxPrice(instance store.InstanceType, maxPrice float64) bool {
	price, err := strconv.ParseFloat(instance.BasePrice.Amount, 64)
	if err != nil {
		return false
	}
	return price <= maxPrice
}

func hasCapabilities(instance store.InstanceType, capabilities []string) bool {
	for _, cap := range capabilities {
		switch strings.ToLower(strings.TrimSpace(cap)) {
		case "stoppable":
			if !instance.Stoppable {
				return false
			}
		case "rebootable":
			if !instance.Rebootable {
				return false
			}
		case "firewall":
			if !instance.CanModifyFirewall {
				return false
			}
		}
	}
	return true
}

func hasMinRAM(instance store.InstanceType, minRAM int) bool {
	ram := parseMemoryToGB(instance.Memory)
	return ram >= minRAM
}

func hasMinCPU(instance store.InstanceType, minCPU int) bool {
	return instance.VCPU >= minCPU || instance.DefaultCores >= minCPU
}

func parseMemoryToGB(memStr string) int {
	memStr = strings.TrimSpace(memStr)
	if memStr == "" {
		return 0
	}

	if strings.HasSuffix(memStr, "GiB") || strings.HasSuffix(memStr, "GB") {
		numStr := strings.TrimSuffix(strings.TrimSuffix(memStr, "GiB"), "GB")
		if val, err := strconv.Atoi(numStr); err == nil {
			return val
		}
	}

	if strings.HasSuffix(memStr, "TiB") || strings.HasSuffix(memStr, "TB") {
		numStr := strings.TrimSuffix(strings.TrimSuffix(memStr, "TiB"), "TB")
		if val, err := strconv.Atoi(numStr); err == nil {
			return val * 1024
		}
	}

	if strings.HasSuffix(memStr, "MiB") || strings.HasSuffix(memStr, "MB") {
		numStr := strings.TrimSuffix(strings.TrimSuffix(memStr, "MiB"), "MB")
		if val, err := strconv.Atoi(numStr); err == nil {
			return val / 1024
		}
	}

	return 0
}

func sortInstanceTypesByPrice(instances []store.InstanceType) {
	sort.Slice(instances, func(i, j int) bool {
		priceI, errI := strconv.ParseFloat(instances[i].BasePrice.Amount, 64)
		priceJ, errJ := strconv.ParseFloat(instances[j].BasePrice.Amount, 64)

		if errI != nil {
			return false
		}
		if errJ != nil {
			return true
		}

		return priceI < priceJ
	})
}

func displayInstanceTypesTable(t *terminal.Terminal, instances []store.InstanceType) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()

	header := table.Row{"Instance Type", "Provider", "GPUs", "Node VRAM", "Memory", "vCPUs", "Price/hr", "Capabilities"}
	ta.AppendHeader(header)

	for _, instance := range instances {
		gpuInfo := formatGPUInfo(instance.SupportedGPUs)
		nodeVRAM := formatNodeVRAM(instance.SupportedGPUs)
		capabilities := formatCapabilities(instance)
		price := fmt.Sprintf("$%s", instance.BasePrice.Amount)

		row := table.Row{
			instance.Type,
			instance.Provider,
			gpuInfo,
			nodeVRAM,
			instance.Memory,
			fmt.Sprintf("%d", instance.VCPU),
			price,
			capabilities,
		}
		ta.AppendRow(row)
	}

	ta.Render()
}

func formatGPUInfo(gpus []store.GPU) string {
	if len(gpus) == 0 {
		return "None"
	}

	var parts []string
	for _, gpu := range gpus {
		if gpu.Count > 1 {
			parts = append(parts, fmt.Sprintf("%dx %s (%s)", gpu.Count, gpu.Name, gpu.Memory))
		} else {
			parts = append(parts, fmt.Sprintf("%s (%s)", gpu.Name, gpu.Memory))
		}
	}

	return strings.Join(parts, ", ")
}

func formatNodeVRAM(gpus []store.GPU) string {
	if len(gpus) == 0 {
		return "0GB"
	}

	totalVRAM := 0
	for _, gpu := range gpus {
		vram := parseMemoryToGB(gpu.Memory)
		totalVRAM += vram * gpu.Count
	}

	if totalVRAM == 0 {
		return "0GB"
	}

	return fmt.Sprintf("%dGB", totalVRAM)
}

func formatCapabilities(instance store.InstanceType) string {
	var caps []string
	if instance.Stoppable {
		caps = append(caps, "stoppable")
	}
	if instance.Rebootable {
		caps = append(caps, "rebootable")
	}
	if instance.CanModifyFirewall {
		caps = append(caps, "firewall")
	}

	if len(caps) == 0 {
		return "None"
	}

	return strings.Join(caps, ", ")
}

func getBrevTableOptions() table.Options {
	options := table.OptionsDefault
	options.DrawBorder = false
	options.SeparateColumns = false
	options.SeparateRows = false
	options.SeparateHeader = false
	return options
}
