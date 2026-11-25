package gpusearch

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarizeLocations(t *testing.T) {
	instances := ProcessInstances([]InstanceType{
		{Type: "cpu-a", Provider: "gcp", AvailableLocations: []string{"us-west2", "asia-south1"}},
		{Type: "cpu-b", Provider: "aws", AvailableLocations: []string{"us-west2"}},
		{
			// Two GPU entries produce two rows; the type is counted once.
			Type: "gpu-a", Provider: "gcp", AvailableLocations: []string{"us-west2"},
			SupportedGPUs: []GPU{{Count: 1, Name: "L4", Manufacturer: "NVIDIA"}, {Count: 2, Name: "L4", Manufacturer: "NVIDIA"}},
		},
		{Type: "cpu-c", Provider: "nebius", AvailableLocations: []string{"us-west2"}},
		{Type: "cpu-d", Provider: "lambda", AvailableLocations: []string{"us-west2"}},
		{Type: "no-provider", AvailableLocations: []string{"unknown-1"}},
		{Type: "no-locations", Provider: "aws"},
	})

	want := []LocationInfo{
		{Location: "asia-south1", Providers: []string{"gcp"}, GPUTypes: 0, CPUTypes: 1},
		{Location: "unknown-1", Providers: []string{}, GPUTypes: 0, CPUTypes: 1},
		{Location: "us-west2", Providers: []string{"aws", "gcp", "lambda", "nebius"}, GPUTypes: 1, CPUTypes: 4},
	}
	for range 5 { // map iteration order varies between runs
		assert.Equal(t, want, SummarizeLocations(instances))
	}
	assert.Empty(t, SummarizeLocations(nil))
}

func TestLocationsTableShowsDashForUnknownProvider(t *testing.T) {
	store := &trackingStore{Response: &InstanceTypesResponse{Items: []InstanceType{
		{Type: "no-provider", AvailableLocations: []string{"unknown-1"}},
	}}}
	cmd := NewCmdGPUSearch(terminal.New(), store)
	cmd.SetArgs([]string{"locations"})
	out := captureStdout(t, func() { require.NoError(t, cmd.Execute()) })
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, []string{"unknown-1", "-", "0", "1"}, strings.Fields(lines[1]))
}

func TestSearchLocationsCommand(t *testing.T) {
	catalog := locationCatalog()
	catalog.Items = append(catalog.Items, InstanceType{
		Type: "amd-gpu", Provider: "aws", AvailableLocations: []string{"us-east-1"},
		SupportedGPUs: []GPU{{Count: 1, Name: "MI300X", Manufacturer: "AMD"}},
	})
	run := func(args ...string) (string, *trackingStore) {
		store := &trackingStore{Response: catalog}
		cmd := NewCmdGPUSearch(terminal.New(), store)
		cmd.SetArgs(append([]string{"locations"}, args...))
		return captureStdout(t, func() { require.NoError(t, cmd.Execute()) }), store
	}
	parse := func(out string) []LocationInfo {
		var got []LocationInfo
		require.NoError(t, json.Unmarshal([]byte(out), &got))
		return got
	}

	t.Run("lists every location with GPU and CPU type counts", func(t *testing.T) {
		out, store := run("--json")
		assert.Equal(t, []bool{true}, store.calls, "needs the catalog with CPU types")
		assert.Equal(t, []LocationInfo{
			{Location: "asia-south1", Providers: []string{"gcp"}, GPUTypes: 0, CPUTypes: 1},
			{Location: "us-east-1", Providers: []string{"aws"}, GPUTypes: 1, CPUTypes: 1},
			{Location: "us-west-2", Providers: []string{"aws"}, GPUTypes: 1, CPUTypes: 0},
			{Location: "us-west2", Providers: []string{"gcp"}, GPUTypes: 0, CPUTypes: 1},
		}, parse(out), "non-NVIDIA GPUs are excluded, as in 'brev search gpu'")
	})

	t.Run("--type lists where one type can be created", func(t *testing.T) {
		out, _ := run("--type", "n2d-standard-2", "--json")
		var locations []string
		for _, loc := range parse(out) {
			locations = append(locations, loc.Location)
		}
		assert.Equal(t, []string{"asia-south1", "us-west2"}, locations)
	})

	t.Run("--provider filters", func(t *testing.T) {
		out, _ := run("--provider", "gcp", "--json")
		for _, loc := range parse(out) {
			assert.Equal(t, []string{"gcp"}, loc.Providers)
		}
		assert.Len(t, parse(out), 2)
	})

	t.Run("no match reports empty results", func(t *testing.T) {
		out, _ := run("--type", "does-not-exist", "--json")
		assert.Equal(t, "[]\n", out)
	})

	t.Run("piped table", func(t *testing.T) {
		out, _ := run()
		lines := strings.Split(strings.TrimSpace(out), "\n")
		require.Len(t, lines, 5)
		assert.Equal(t, []string{"LOCATION", "PROVIDERS", "GPU_TYPES", "CPU_TYPES"}, strings.Fields(lines[0]))
		assert.Equal(t, []string{"us-east-1", "aws", "1", "1"}, strings.Fields(lines[2]))
	})

	t.Run("counts match search --location", func(t *testing.T) {
		out, _ := run("--json")
		for _, loc := range parse(out) {
			gpus, _ := searchTypes(t, "gpu", "--location", loc.Location)
			cpus, _ := searchTypes(t, "cpu", "--location", loc.Location)
			assert.Len(t, gpus, loc.GPUTypes, "GPU types in %s", loc.Location)
			assert.Len(t, cpus, loc.CPUTypes, "CPU types in %s", loc.Location)
		}
	})
}
