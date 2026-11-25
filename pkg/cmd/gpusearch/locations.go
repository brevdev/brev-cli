package gpusearch

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

const locationsExample = `
  # List every location and how many types can be created there
  brev search locations

  # Where can a type be created?
  brev search locations --type n2d-standard-2

  # Locations for one provider
  brev search locations --provider aws

  # Then search or create in a location
  brev search cpu --location us-west2
  brev create my-instance --type n2d-standard-2 --location us-west2
`

// LocationInfo summarizes the instance types that can be created in a location.
type LocationInfo struct {
	Location  string   `json:"location"`
	Providers []string `json:"providers"`
	GPUTypes  int      `json:"gpu_types"`
	CPUTypes  int      `json:"cpu_types"`
}

func newCmdLocationsSubcommand(t *terminal.Terminal, store GPUSearchStore) *cobra.Command {
	var instanceType, provider string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:                   "locations",
		DisableFlagsInUseLine: true,
		Short:                 "List locations that instance types can be created in",
		Long: `List the locations that instance types can be created in, with how many GPU
and CPU types are available in each. Pass a location to 'brev search --location'
or 'brev create --location'.`,
		Example: locationsExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunLocationSearch(t, store, instanceType, provider, jsonOutput)
		},
	}

	cmd.Flags().StringVar(&instanceType, "type", "", "Only show locations this instance type can be created in (exact match)")
	cmd.Flags().StringVarP(&provider, "provider", "p", "", "Filter by provider/cloud (case-insensitive, partial match)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")

	return cmd
}

// RunLocationSearch lists the locations of the instance types that match the filters.
func RunLocationSearch(t *terminal.Terminal, store GPUSearchStore, instanceType, provider string, jsonOutput bool) error {
	response, err := store.GetInstanceTypes(true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	var items []InstanceType
	if response != nil {
		items = response.Items
	}

	// The same filtering as 'brev search gpu' and 'brev search cpu', so the
	// counts match what those commands list with --location.
	instances := FilterInstances(ProcessInstances(items), "", provider, "", 0, 0, 0, 0, 0, 0, 0, false, false, false, false)
	if instanceType != "" {
		instances = slices.DeleteFunc(instances, func(inst GPUInstanceInfo) bool { return inst.Type != instanceType })
	}

	locations := SummarizeLocations(instances)
	if len(locations) == 0 {
		return displayEmptyResults(t, "No locations match the specified filters", jsonOutput, IsStdoutPiped())
	}
	if jsonOutput {
		output, err := json.MarshalIndent(locations, "", "  ")
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		fmt.Println(string(output))
		return nil
	}

	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()
	ta.AppendHeader(table.Row{"LOCATION", "PROVIDERS", "GPU_TYPES", "CPU_TYPES"})
	for _, loc := range locations {
		ta.AppendRow(table.Row{loc.Location, joinOrDash(loc.Providers), loc.GPUTypes, loc.CPUTypes})
	}
	ta.Render()
	if !IsStdoutPiped() {
		t.Vprintf("\n%s\n", t.Green(fmt.Sprintf("Found %d locations", len(locations))))
	}
	return nil
}

// SummarizeLocations groups instances by every location they are available
// in, counting distinct GPU and CPU types. Results are sorted by location.
func SummarizeLocations(instances []GPUInstanceInfo) []LocationInfo {
	type typeSets struct{ gpu, cpu, providers map[string]bool }
	byLocation := map[string]*typeSets{}
	for _, inst := range instances {
		for _, location := range inst.AvailableLocations {
			sets := byLocation[location]
			if sets == nil {
				sets = &typeSets{gpu: map[string]bool{}, cpu: map[string]bool{}, providers: map[string]bool{}}
				byLocation[location] = sets
			}
			if inst.Manufacturer == "cpu" {
				sets.cpu[inst.Type] = true
			} else {
				sets.gpu[inst.Type] = true
			}
			if inst.Provider != "" {
				sets.providers[inst.Provider] = true
			}
		}
	}

	locations := make([]LocationInfo, 0, len(byLocation))
	for location, sets := range byLocation {
		providers := make([]string, 0, len(sets.providers))
		for p := range sets.providers {
			providers = append(providers, p)
		}
		slices.Sort(providers)
		locations = append(locations, LocationInfo{
			Location:  location,
			Providers: providers,
			GPUTypes:  len(sets.gpu),
			CPUTypes:  len(sets.cpu),
		})
	}
	slices.SortFunc(locations, func(a, b LocationInfo) int { return strings.Compare(a.Location, b.Location) })
	return locations
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}
