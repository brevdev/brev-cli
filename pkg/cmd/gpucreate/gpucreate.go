// Package gpucreate provides a command to create GPU instances with retry logic
package gpucreate

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	long = `Create GPU instances with automatic retry across multiple instance types.

This command attempts to create GPU instances, trying different instance types
until the desired number of instances are successfully created. Instance types
can be specified directly, piped from 'brev search', or auto-selected using defaults.

Search Filters:
You can use the same filter flags as 'brev search' to control which GPU types
are considered. If no instance types are specified (no --type flag and no piped input),
the command automatically searches for GPUs matching either your filters or defaults:
  - Minimum 20GB total VRAM (--min-total-vram)
  - Minimum 500GB disk (--min-disk)
  - Compute capability 8.0+ (--min-capability)
  - Boot time under 7 minutes (--max-boot-time, only when no filters set)
Results are sorted by price (cheapest first) unless --sort is specified.

Retry and Fallback Logic:
When multiple instance types are provided (via --type or piped input), the command
tries to create ALL instances using the first type before falling back to the next:

  1. Try first type for all instances (using --parallel workers if specified)
  2. If first type succeeds for all instances, done
  3. If first type fails for some instances, try second type for remaining instances
  4. Continue until all instances are created or all types are exhausted

Example with --count 2 and types [A, B]:
  - Try A for instance-1 → success
  - Try A for instance-2 → success
  - Done! (both instances use type A)

If type A fails for instance-2:
  - Try A for instance-1 → success
  - Try A for instance-2 → fail
  - Try B for instance-2 → success
  - Done! (instance-1 uses A, instance-2 uses B)

Startup Scripts:
You can attach a startup script that runs when the instance boots using the
--startup-script flag. The script can be provided as:
  - An inline string: --startup-script 'pip install torch'
  - A file path (prefix with @): --startup-script @setup.sh
  - An absolute file path: --startup-script @/path/to/setup.sh`

	example = `
  # Create an instance using smart defaults (sorted by price)
  brev create my-instance

  # Create with a specific GPU type
  brev create my-instance --type g5.xlarge

  # Try multiple types in order (fallback chain)
  brev create my-instance --type g5.xlarge,g5.2xlarge,g4dn.xlarge

  # Pipe from search for automatic fallback
  brev search --gpu-name A100 | brev create my-instance

  # Create multiple instances in parallel
  brev create my-cluster --count 3 --type g5.xlarge --parallel 3

  # Use search filters directly and attach a startup script
  brev create my-instance -g a100 --startup-script @setup.sh
`
)

// GPUCreateStore defines the interface for GPU create operations
type GPUCreateStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	gpusearch.GPUSearchStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	DeleteWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllInstanceTypesWithWorkspaceGroups(orgID string) (*gpusearch.AllInstanceTypesResponse, error)
}

// Default filter values for automatic GPU selection
const (
	defaultMinTotalVRAM  = 20.0  // GB
	defaultMinDisk       = 500.0 // GB
	defaultMinCapability = 8.0
	defaultMaxBootTime   = 7 // minutes
)

// CreateResult holds the result of a workspace creation attempt
type CreateResult struct {
	Workspace    *entity.Workspace
	InstanceType string
	Error        error
}

// searchFilterFlags holds the search filter flag values for create
type searchFilterFlags struct {
	gpuName       string
	provider      string
	minVRAM       float64
	minTotalVRAM  float64
	minCapability float64
	minDisk       float64
	maxBootTime   int
	stoppable     bool
	rebootable    bool
	flexPorts     bool
	sortBy        string
	descending    bool
}

// hasUserFilters returns true if the user specified any search filter flags
func (f *searchFilterFlags) hasUserFilters() bool {
	return f.gpuName != "" || f.provider != "" || f.minVRAM > 0 || f.minTotalVRAM > 0 ||
		f.minCapability > 0 || f.minDisk > 0 || f.maxBootTime > 0 ||
		f.stoppable || f.rebootable || f.flexPorts
}

// NewCmdGPUCreate creates the gpu-create command
func NewCmdGPUCreate(t *terminal.Terminal, gpuCreateStore GPUCreateStore) *cobra.Command {
	var name string
	var instanceTypes string
	var count int
	var parallel int
	var detached bool
	var timeout int
	var startupScript string
	var dryRun bool
	var filters searchFilterFlags

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "create [name]",
		Aliases:               []string{"provision", "gpu-create", "gpu-retry", "gcreate"},
		DisableFlagsInUseLine: true,
		Short:                 "Create GPU instances with automatic retry",
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Accept name as positional arg or --name flag
			if len(args) > 0 && name == "" {
				name = args[0]
			}

			// Parse instance types from flag or stdin
			types, err := parseInstanceTypes(instanceTypes)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			// If no types provided, use search filters (or defaults) to find suitable GPUs
			if len(types) == 0 {
				if dryRun {
					return runDryRun(t, gpuCreateStore, &filters)
				}

				types, err = getFilteredInstanceTypes(gpuCreateStore, &filters)
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}

				if len(types) == 0 {
					return breverrors.NewValidationError("no GPU instances match the specified filters. Try 'brev search' to see available options")
				}
			}

			if name == "" {
				return breverrors.NewValidationError("name is required (as argument or --name flag)")
			}

			if count < 1 {
				return breverrors.NewValidationError("--count must be at least 1")
			}

			if parallel < 1 {
				parallel = 1
			}

			// Parse startup script (can be a string or @filepath)
			scriptContent, err := parseStartupScript(startupScript)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			opts := GPUCreateOptions{
				Name:          name,
				InstanceTypes: types,
				Count:         count,
				Parallel:      parallel,
				Detached:      detached,
				Timeout:       time.Duration(timeout) * time.Second,
				StartupScript: scriptContent,
			}

			err = RunGPUCreate(t, gpuCreateStore, opts)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	registerCreateFlags(cmd, &name, &instanceTypes, &count, &parallel, &detached, &timeout, &startupScript, &dryRun, &filters)

	return cmd
}

// registerCreateFlags registers all flags for the create command
func registerCreateFlags(cmd *cobra.Command, name, instanceTypes *string, count, parallel *int, detached *bool, timeout *int, startupScript *string, dryRun *bool, filters *searchFilterFlags) {
	cmd.Flags().StringVarP(name, "name", "n", "", "Base name for the instances (or pass as first argument)")
	cmd.Flags().StringVarP(instanceTypes, "type", "t", "", "Comma-separated list of instance types to try")
	cmd.Flags().IntVarP(count, "count", "c", 1, "Number of instances to create")
	cmd.Flags().IntVarP(parallel, "parallel", "p", 1, "Number of parallel creation attempts")
	cmd.Flags().BoolVarP(detached, "detached", "d", false, "Don't wait for instances to be ready")
	cmd.Flags().IntVar(timeout, "timeout", 300, "Timeout in seconds for each instance to become ready")
	cmd.Flags().StringVarP(startupScript, "startup-script", "s", "", "Startup script to run on instance (string or @filepath)")
	cmd.Flags().BoolVar(dryRun, "dry-run", false, "Show matching instance types without creating anything")

	cmd.Flags().StringVarP(&filters.gpuName, "gpu-name", "g", "", "Filter by GPU name (e.g., A100, H100)")
	cmd.Flags().StringVar(&filters.provider, "provider", "", "Filter by provider/cloud (e.g., aws, gcp)")
	cmd.Flags().Float64VarP(&filters.minVRAM, "min-vram", "v", 0, "Minimum VRAM per GPU in GB")
	cmd.Flags().Float64Var(&filters.minTotalVRAM, "min-total-vram", 0, "Minimum total VRAM in GB")
	cmd.Flags().Float64Var(&filters.minCapability, "min-capability", 0, "Minimum GPU compute capability (e.g., 8.0)")
	cmd.Flags().Float64Var(&filters.minDisk, "min-disk", 0, "Minimum disk size in GB")
	cmd.Flags().IntVar(&filters.maxBootTime, "max-boot-time", 0, "Maximum boot time in minutes")
	cmd.Flags().BoolVar(&filters.stoppable, "stoppable", false, "Only use instances that can be stopped/restarted")
	cmd.Flags().BoolVar(&filters.rebootable, "rebootable", false, "Only use instances that can be rebooted")
	cmd.Flags().BoolVar(&filters.flexPorts, "flex-ports", false, "Only use instances with configurable firewall rules")
	cmd.Flags().StringVar(&filters.sortBy, "sort", "price", "Sort instance preference by: price, vram, boot-time, etc.")
	cmd.Flags().BoolVar(&filters.descending, "desc", false, "Sort in descending order")
}

// InstanceSpec holds an instance type and its target disk size
type InstanceSpec struct {
	Type   string
	DiskGB float64 // Target disk size in GB, 0 means use default
}

// GPUCreateOptions holds the options for GPU instance creation
type GPUCreateOptions struct {
	Name          string
	InstanceTypes []InstanceSpec
	Count         int
	Parallel      int
	Detached      bool
	Timeout       time.Duration
	StartupScript string
}

// parseStartupScript parses the startup script from a string or file path
// If the value starts with @, it's treated as a file path
func parseStartupScript(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	// Check if it's a file path (prefixed with @)
	if strings.HasPrefix(value, "@") {
		filePath := strings.TrimPrefix(value, "@")
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		return string(content), nil
	}

	// Otherwise, treat it as the script content directly
	return value, nil
}

// searchInstances fetches and filters GPU instances using user-provided filters merged with defaults
func searchInstances(s GPUCreateStore, filters *searchFilterFlags) ([]gpusearch.GPUInstanceInfo, float64, error) {
	response, err := s.GetInstanceTypes()
	if err != nil {
		return nil, 0, breverrors.WrapAndTrace(err)
	}

	if response == nil || len(response.Items) == 0 {
		return nil, 0, nil
	}

	minTotalVRAM := orDefault(filters.minTotalVRAM, defaultMinTotalVRAM)
	minCapability := orDefault(filters.minCapability, defaultMinCapability)
	minDisk := orDefault(filters.minDisk, defaultMinDisk)
	maxBootTime := filters.maxBootTime
	if maxBootTime == 0 && !filters.hasUserFilters() {
		maxBootTime = defaultMaxBootTime
	}
	sortBy := filters.sortBy
	if sortBy == "" {
		sortBy = "price"
	}

	instances := gpusearch.ProcessInstances(response.Items)
	filtered := gpusearch.FilterInstances(instances, filters.gpuName, filters.provider, filters.minVRAM,
		minTotalVRAM, minCapability, minDisk, maxBootTime, filters.stoppable, filters.rebootable, filters.flexPorts)
	gpusearch.SortInstances(filtered, sortBy, filters.descending)

	return filtered, minDisk, nil
}

// getFilteredInstanceTypes fetches GPU instance types using user-provided filters
// merged with defaults. When a filter flag is not set, the default value is used.
func getFilteredInstanceTypes(s GPUCreateStore, filters *searchFilterFlags) ([]InstanceSpec, error) {
	filtered, minDisk, err := searchInstances(s, filters)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var specs []InstanceSpec
	for _, inst := range filtered {
		diskGB := inst.DiskMin
		if inst.DiskMin != inst.DiskMax && minDisk > inst.DiskMin && minDisk <= inst.DiskMax {
			diskGB = minDisk
		}
		specs = append(specs, InstanceSpec{Type: inst.Type, DiskGB: diskGB})
	}

	return specs, nil
}

// runDryRun shows the instance types that would be used without creating anything
func runDryRun(t *terminal.Terminal, s GPUCreateStore, filters *searchFilterFlags) error {
	filtered, _, err := searchInstances(s, filters)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	piped := gpusearch.IsStdoutPiped()
	if err := gpusearch.DisplayResults(t, filtered, false, piped); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// orDefault returns val if it's non-zero, otherwise returns def
func orDefault(val, def float64) float64 {
	if val > 0 {
		return val
	}
	return def
}

// parseInstanceTypes parses instance types from flag value or stdin
// Returns InstanceSpec with type and optional disk size (from JSON input)
func parseInstanceTypes(flagValue string) ([]InstanceSpec, error) {
	var specs []InstanceSpec

	// First check if there's a flag value
	if flagValue != "" {
		parts := strings.Split(flagValue, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				specs = append(specs, InstanceSpec{Type: p})
			}
		}
	}

	// Check if there's piped input from stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped to stdin - read all input first
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}

		inputStr := strings.TrimSpace(string(input))
		if inputStr == "" {
			return specs, nil
		}

		// Check if input is JSON (starts with '[')
		if strings.HasPrefix(inputStr, "[") {
			jsonSpecs, err := parseJSONInput(inputStr)
			if err != nil {
				return nil, breverrors.WrapAndTrace(err)
			}
			specs = append(specs, jsonSpecs...)
		} else {
			// Parse as table format
			tableSpecs := parseTableInput(inputStr)
			specs = append(specs, tableSpecs...)
		}
	}

	return specs, nil
}

// parseJSONInput parses JSON array input from gpu-search --json
func parseJSONInput(input string) ([]InstanceSpec, error) {
	var instances []gpusearch.GPUInstanceInfo
	if err := json.Unmarshal([]byte(input), &instances); err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var specs []InstanceSpec
	for _, inst := range instances {
		spec := InstanceSpec{
			Type:   inst.Type,
			DiskGB: inst.TargetDisk,
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// parseTableInput parses table format input from gpu-search
// Table format: TYPE  TARGET_DISK  PROVIDER  GPU  COUNT  ...
func parseTableInput(input string) []InstanceSpec {
	var specs []InstanceSpec
	lines := strings.Split(input, "\n")

	for i, line := range lines {
		// Skip header line (first line typically contains column names)
		if i == 0 && (strings.Contains(line, "TYPE") || strings.Contains(line, "GPU")) {
			continue
		}

		// Skip empty lines
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip summary lines (e.g., "Found X GPU instance types")
		if strings.HasPrefix(line, "Found ") {
			continue
		}

		// Extract TYPE (column 0) and TARGET_DISK (column 1) from the table output
		// The format is: TYPE  TARGET_DISK  PROVIDER  GPU  COUNT  ...
		fields := strings.Fields(line)
		if len(fields) > 0 {
			instanceType := fields[0]
			// Validate it looks like an instance type (contains letters and possibly numbers/dots)
			if isValidInstanceType(instanceType) {
				spec := InstanceSpec{Type: instanceType}
				// Parse TARGET_DISK if present (column 1)
				if len(fields) > 1 {
					if diskGB, err := strconv.ParseFloat(fields[1], 64); err == nil && diskGB > 0 {
						spec.DiskGB = diskGB
					}
				}
				specs = append(specs, spec)
			}
		}
	}

	return specs
}

// isValidInstanceType checks if a string looks like a valid instance type.
// Instance types typically have formats like: g5.xlarge, p4d.24xlarge, n1-highmem-4:nvidia-tesla-t4:1
func isValidInstanceType(s string) bool {
	if len(s) < 2 {
		return false
	}
	var hasLetter, hasDigit bool
	for _, c := range s {
		if unicode.IsLetter(c) {
			hasLetter = true
		} else if unicode.IsDigit(c) {
			hasDigit = true
		}
		if hasLetter && hasDigit {
			return true
		}
	}
	return hasLetter && hasDigit
}

// formatInstanceSpecs formats a slice of InstanceSpec for display
func formatInstanceSpecs(specs []InstanceSpec) string {
	var parts []string
	for _, spec := range specs {
		if spec.DiskGB > 0 {
			parts = append(parts, fmt.Sprintf("%s (%.0fGB disk)", spec.Type, spec.DiskGB))
		} else {
			parts = append(parts, spec.Type)
		}
	}
	return strings.Join(parts, ", ")
}

// createContext holds shared state for instance creation
type createContext struct {
	t                *terminal.Terminal
	store            GPUCreateStore
	opts             GPUCreateOptions
	org              *entity.Organization
	user             *entity.User
	allInstanceTypes *gpusearch.AllInstanceTypesResponse
	piped            bool
	logf             func(format string, a ...interface{})
}

// newCreateContext initializes the context for instance creation
func newCreateContext(t *terminal.Terminal, store GPUCreateStore, opts GPUCreateOptions) (*createContext, error) {
	piped := gpusearch.IsStdoutPiped()

	ctx := &createContext{
		t:     t,
		store: store,
		opts:  opts,
		piped: piped,
	}

	// Set up logging function
	ctx.logf = func(format string, a ...interface{}) {
		if piped {
			fmt.Fprintf(os.Stderr, format, a...)
		} else {
			t.Vprintf(format, a...)
		}
	}

	// Get user
	user, err := store.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	ctx.user = user

	// Get organization
	org, err := store.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, breverrors.NewValidationError("no organization found")
	}
	ctx.org = org

	// Fetch instance types with workspace groups
	allInstanceTypes, err := store.GetAllInstanceTypesWithWorkspaceGroups(org.ID)
	if err != nil {
		ctx.logf("Warning: could not fetch instance types with workspace groups: %s\n", err.Error())
		ctx.logf("Falling back to default workspace group\n")
	}
	ctx.allInstanceTypes = allInstanceTypes

	return ctx, nil
}

// typeCreateResult holds the result of creating instances with a single type
type typeCreateResult struct {
	successes  []*entity.Workspace
	hadFailure bool
	fatalError error
}

// createInstancesWithType attempts to create instances using a specific type
func (c *createContext) createInstancesWithType(spec InstanceSpec, startIdx, count int) typeCreateResult {
	result := typeCreateResult{}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// Determine worker count
	workerCount := c.opts.Parallel
	if workerCount > count {
		workerCount = count
	}

	// Create index channel
	indicesToCreate := make(chan int, count)
	for i := startIdx; i < startIdx+count; i++ {
		indicesToCreate <- i
	}
	close(indicesToCreate)

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.runWorker(workerID, spec, indicesToCreate, &result, &mu)
		}(i)
	}

	wg.Wait()
	return result
}

// runWorker processes instance creation requests from the channel
func (c *createContext) runWorker(workerID int, spec InstanceSpec, indices <-chan int, result *typeCreateResult, mu *sync.Mutex) {
	for idx := range indices {
		// Check if we've already created enough
		mu.Lock()
		if len(result.successes) >= c.opts.Count {
			mu.Unlock()
			return
		}
		mu.Unlock()

		// Determine instance name
		instanceName := c.opts.Name
		if c.opts.Count > 1 {
			instanceName = fmt.Sprintf("%s-%d", c.opts.Name, idx+1)
		}

		c.logf("[Worker %d] Trying %s for instance '%s'...\n", workerID+1, spec.Type, instanceName)

		workspace, err := c.createWorkspace(instanceName, spec)

		mu.Lock()
		if err != nil {
			c.handleCreateError(workerID, spec.Type, instanceName, err, result)
		} else {
			c.handleCreateSuccess(workerID, spec.Type, instanceName, workspace, result)
		}
		mu.Unlock()
	}
}

// colorize applies a terminal color function if output is not piped
func (c *createContext) colorize(s string, colorFn func(string, ...interface{}) string) string {
	if c.piped {
		return s
	}
	return colorFn(s)
}

// handleCreateError processes a failed instance creation (must be called with lock held)
func (c *createContext) handleCreateError(workerID int, instanceType, instanceName string, err error, result *typeCreateResult) {
	errStr := err.Error()
	c.logf("[Worker %d] %s Failed: %s\n", workerID+1, c.colorize(instanceType, c.t.Yellow), errStr)

	result.hadFailure = true
	if strings.Contains(errStr, "duplicate workspace") {
		result.fatalError = fmt.Errorf("workspace '%s' already exists. Use a different name or delete the existing workspace", instanceName)
	}
}

// handleCreateSuccess processes a successful instance creation (must be called with lock held)
func (c *createContext) handleCreateSuccess(workerID int, instanceType, instanceName string, workspace *entity.Workspace, result *typeCreateResult) {
	c.logf("[Worker %d] %s Success! Created instance '%s'\n", workerID+1, c.colorize(instanceType, c.t.Green), instanceName)
	result.successes = append(result.successes, workspace)
}

// cleanupExtraInstances deletes instances beyond the requested count
func (c *createContext) cleanupExtraInstances(workspaces []*entity.Workspace) []*entity.Workspace {
	if len(workspaces) <= c.opts.Count {
		return workspaces
	}

	extras := workspaces[c.opts.Count:]
	c.logf("\nCleaning up %d extra instance(s)...\n", len(extras))

	for _, ws := range extras {
		c.logf("  Deleting %s...", ws.Name)
		_, err := c.store.DeleteWorkspace(ws.ID)
		if err != nil {
			c.logf(" Failed\n")
		} else {
			c.logf(" Done\n")
		}
	}

	return workspaces[:c.opts.Count]
}

// waitForInstances waits for all instances to be ready
func (c *createContext) waitForInstances(workspaces []*entity.Workspace) {
	if c.opts.Detached {
		return
	}

	c.logf("\nWaiting for instance(s) to be ready...\n")
	c.logf("You can safely ctrl+c to exit\n")

	for _, ws := range workspaces {
		err := c.pollUntilReady(ws.ID)
		if err != nil {
			c.logf("  %s: Timeout waiting for ready state\n", ws.Name)
		}
	}
}

// printSummary outputs the final creation summary
func (c *createContext) printSummary(workspaces []*entity.Workspace) {
	if c.piped {
		for _, ws := range workspaces {
			fmt.Println(ws.Name)
		}
		return
	}

	fmt.Print("\n")
	c.t.Vprint(c.t.Green(fmt.Sprintf("Successfully created %d instance(s)!\n\n", len(workspaces))))

	for _, ws := range workspaces {
		c.t.Vprintf("Instance: %s\n", c.t.Green(ws.Name))
		c.t.Vprintf("  ID: %s\n", ws.ID)
		c.t.Vprintf("  Type: %s\n", ws.InstanceType)
		displayConnectBreadCrumb(c.t, ws)
		fmt.Print("\n")
	}
}

// RunGPUCreate executes the GPU create with retry logic
func RunGPUCreate(t *terminal.Terminal, gpuCreateStore GPUCreateStore, opts GPUCreateOptions) error {
	ctx, err := newCreateContext(t, gpuCreateStore, opts)
	if err != nil {
		return err
	}

	ctx.logf("Attempting to create %d instance(s) with %d parallel attempts\n", opts.Count, opts.Parallel)
	ctx.logf("Instance types to try: %s\n\n", formatInstanceSpecs(opts.InstanceTypes))

	var successfulWorkspaces []*entity.Workspace

	// Try each instance type in order
	for _, spec := range opts.InstanceTypes {
		if len(successfulWorkspaces) >= opts.Count {
			break
		}

		remaining := opts.Count - len(successfulWorkspaces)
		ctx.logf("Trying %s for %d instance(s)...\n", spec.Type, remaining)

		result := ctx.createInstancesWithType(spec, len(successfulWorkspaces), remaining)
		successfulWorkspaces = append(successfulWorkspaces, result.successes...)

		if result.fatalError != nil {
			ctx.logf("\nError: %s\n", result.fatalError.Error())
			break
		}

		if !result.hadFailure && len(successfulWorkspaces) >= opts.Count {
			break
		}

		if len(successfulWorkspaces) < opts.Count && result.hadFailure {
			ctx.logf("\nType %s had failures, trying next type...\n\n", spec.Type)
		}
	}

	// Check if we created enough instances
	if len(successfulWorkspaces) < opts.Count {
		ctx.logf("\nWarning: Only created %d/%d instances\n", len(successfulWorkspaces), opts.Count)
		if len(successfulWorkspaces) > 0 {
			ctx.logf("Successfully created instances:\n")
			for _, ws := range successfulWorkspaces {
				ctx.logf("  - %s (ID: %s)\n", ws.Name, ws.ID)
			}
		}
		return breverrors.NewValidationError(fmt.Sprintf("could only create %d/%d instances", len(successfulWorkspaces), opts.Count))
	}

	successfulWorkspaces = ctx.cleanupExtraInstances(successfulWorkspaces)
	ctx.waitForInstances(successfulWorkspaces)
	ctx.printSummary(successfulWorkspaces)

	return nil
}

// createWorkspace creates a workspace with the specified instance type and name
func (c *createContext) createWorkspace(name string, spec InstanceSpec) (*entity.Workspace, error) {
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	cwOptions := store.NewCreateWorkspacesOptions(clusterID, name)
	cwOptions.WithInstanceType(spec.Type)
	cwOptions = resolveWorkspaceUserOptions(cwOptions, c.user)

	if spec.DiskGB > 0 {
		cwOptions.DiskStorage = fmt.Sprintf("%.0fGi", spec.DiskGB)
	}

	if c.allInstanceTypes != nil {
		if wgID := c.allInstanceTypes.GetWorkspaceGroupID(spec.Type); wgID != "" {
			cwOptions.WorkspaceGroupID = wgID
		}
	}

	if c.opts.StartupScript != "" {
		cwOptions.VMBuild = &store.VMBuild{
			ForceJupyterInstall: true,
			LifeCycleScriptAttr: &store.LifeCycleScriptAttr{
				Script: c.opts.StartupScript,
			},
		}
	}

	workspace, err := c.store.CreateWorkspace(c.org.ID, cwOptions)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return workspace, nil
}

// resolveWorkspaceUserOptions sets workspace template and class based on user type
func resolveWorkspaceUserOptions(options *store.CreateWorkspacesOptions, user *entity.User) *store.CreateWorkspacesOptions {
	isAdmin := featureflag.IsAdmin(user.GlobalUserType)
	if options.WorkspaceTemplateID == "" {
		options.WorkspaceTemplateID = store.UserWorkspaceTemplateID
		if isAdmin {
			options.WorkspaceTemplateID = store.DevWorkspaceTemplateID
		}
	}
	if options.WorkspaceClassID == "" {
		options.WorkspaceClassID = store.UserWorkspaceClassID
		if isAdmin {
			options.WorkspaceClassID = store.DevWorkspaceClassID
		}
	}
	return options
}

// pollUntilReady waits for a workspace to reach the running state
func (c *createContext) pollUntilReady(wsID string) error {
	deadline := time.Now().Add(c.opts.Timeout)

	for time.Now().Before(deadline) {
		ws, err := c.store.GetWorkspace(wsID)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		if ws.Status == entity.Running {
			c.logf("  %s: %s\n", ws.Name, c.colorize("Ready", c.t.Green))
			return nil
		}

		if ws.Status == entity.Failure {
			return breverrors.NewValidationError(fmt.Sprintf("instance %s failed", ws.Name))
		}

		time.Sleep(5 * time.Second)
	}

	return breverrors.NewValidationError("timeout waiting for instance to be ready")
}

// displayConnectBreadCrumb shows connection instructions
func displayConnectBreadCrumb(t *terminal.Terminal, workspace *entity.Workspace) {
	t.Vprintf("  Connect:\n")
	t.Vprintf("    %s\n", t.Yellow(fmt.Sprintf("brev open %s", workspace.Name)))
	t.Vprintf("    %s\n", t.Yellow(fmt.Sprintf("brev shell %s", workspace.Name)))
}
