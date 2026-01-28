// Package gpucreate provides a command to create GPU instances with retry logic
package gpucreate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
can be specified directly, piped from 'brev gpus', or auto-selected using defaults.

Default Behavior:
If no instance types are specified (no --type flag and no piped input), the command
automatically searches for GPUs matching these criteria:
  - Minimum 20GB total VRAM
  - Minimum 500GB disk
  - Compute capability 8.0+ (Ampere or newer)
  - Boot time under 7 minutes
Results are sorted by price (cheapest first).

The command will:
1. Try to create instances using the provided instance types (in order)
2. Continue until the desired count is reached
3. Optionally try multiple instance types in parallel
4. Clean up any extra instances that were created beyond the requested count

Startup Scripts:
You can attach a startup script that runs when the instance boots using the
--startup-script flag. The script can be provided as:
  - An inline string: --startup-script 'pip install torch'
  - A file path (prefix with @): --startup-script @setup.sh
  - An absolute file path: --startup-script @/path/to/setup.sh`

	example = `
  # Quick start: create an instance using smart defaults (sorted by price)
  brev provision --name my-instance

  # Create and immediately open in VS Code (reads instance name from stdin)
  brev provision --name my-instance | brev open

  # Create and SSH into the instance (use command substitution for interactive shell)
  brev shell $(brev provision --name my-instance)

  # Create and run a command (non-interactive, reads instance name from stdin)
  brev provision --name my-instance | brev shell -c "nvidia-smi"

  # Create a single instance with a specific GPU type
  brev gpu-create --name my-instance --type g5.xlarge

  # Pipe instance types from brev gpus (tries each type until one succeeds)
  brev gpus --min-vram 24 | brev gpu-create --name my-instance

  # Create 3 instances, trying types in parallel
  brev gpus --gpu-name A100 | brev gpu-create --name my-cluster --count 3 --parallel 5

  # Try multiple specific types in order
  brev gpu-create --name my-instance --type g5.xlarge,g5.2xlarge,g4dn.xlarge

  # Attach a startup script from a file
  brev gpu-create --name my-instance --type g5.xlarge --startup-script @setup.sh

  # Attach an inline startup script
  brev gpu-create --name my-instance --type g5.xlarge --startup-script 'pip install torch transformers'

  # Combine: find cheapest A100, attach setup script
  brev gpus --gpu-name A100 --sort price | brev gpu-create --name ml-box --startup-script @ml-setup.sh
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

// NewCmdGPUCreate creates the gpu-create command
func NewCmdGPUCreate(t *terminal.Terminal, gpuCreateStore GPUCreateStore) *cobra.Command {
	var name string
	var instanceTypes string
	var count int
	var parallel int
	var detached bool
	var timeout int
	var startupScript string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "gpu-create",
		Aliases:               []string{"gpu-retry", "gcreate", "provision"},
		DisableFlagsInUseLine: true,
		Short:                 "Create GPU instances with automatic retry",
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if output is being piped (for chaining with brev shell/open)
			piped := isStdoutPiped()

			// Parse instance types from flag or stdin
			types, err := parseInstanceTypes(instanceTypes)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			// If no types provided, use default filters to find suitable GPUs
			if len(types) == 0 {
				msg := fmt.Sprintf("No instance types specified, using defaults: min-total-vram=%.0fGB, min-disk=%.0fGB, min-capability=%.1f, max-boot-time=%dm\n\n",
					defaultMinTotalVRAM, defaultMinDisk, defaultMinCapability, defaultMaxBootTime)
				if piped {
					fmt.Fprint(os.Stderr, msg)
				} else {
					t.Vprint(msg)
				}

				types, err = getDefaultInstanceTypes(gpuCreateStore)
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}

				if len(types) == 0 {
					return breverrors.NewValidationError("no GPU instances match the default filters. Try 'brev gpus' to see available options")
				}
			}

			if name == "" {
				return breverrors.NewValidationError("--name flag is required")
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

	cmd.Flags().StringVarP(&name, "name", "n", "", "Base name for the instances (required)")
	cmd.Flags().StringVarP(&instanceTypes, "type", "t", "", "Comma-separated list of instance types to try")
	cmd.Flags().IntVarP(&count, "count", "c", 1, "Number of instances to create")
	cmd.Flags().IntVarP(&parallel, "parallel", "p", 1, "Number of parallel creation attempts")
	cmd.Flags().BoolVarP(&detached, "detached", "d", false, "Don't wait for instances to be ready")
	cmd.Flags().IntVar(&timeout, "timeout", 300, "Timeout in seconds for each instance to become ready")
	cmd.Flags().StringVarP(&startupScript, "startup-script", "s", "", "Startup script to run on instance (string or @filepath)")

	return cmd
}

// InstanceSpec holds an instance type and its target disk size
type InstanceSpec struct {
	Type     string
	DiskGB   float64 // Target disk size in GB, 0 means use default
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

// getDefaultInstanceTypes fetches GPU instance types using default filters
func getDefaultInstanceTypes(store GPUCreateStore) ([]InstanceSpec, error) {
	response, err := store.GetInstanceTypes()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if response == nil || len(response.Items) == 0 {
		return nil, nil
	}

	// Use gpusearch package to process, filter, and sort instances
	instances := gpusearch.ProcessInstances(response.Items)
	filtered := gpusearch.FilterInstances(instances, "", "", 0, defaultMinTotalVRAM, defaultMinCapability, defaultMinDisk, defaultMaxBootTime)
	gpusearch.SortInstances(filtered, "price", false)

	// Convert to InstanceSpec with disk info
	var specs []InstanceSpec
	for _, inst := range filtered {
		// For defaults, use the minimum disk size that meets the filter
		diskGB := inst.DiskMin
		if inst.DiskMin != inst.DiskMax && defaultMinDisk > inst.DiskMin && defaultMinDisk <= inst.DiskMax {
			diskGB = defaultMinDisk
		}
		specs = append(specs, InstanceSpec{Type: inst.Type, DiskGB: diskGB})
	}

	return specs, nil
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

		// Extract the first column (TYPE) from the table output
		// The format is: TYPE  GPU  COUNT  VRAM/GPU  TOTAL VRAM  CAPABILITY  VCPUs  $/HR
		fields := strings.Fields(line)
		if len(fields) > 0 {
			instanceType := fields[0]
			// Validate it looks like an instance type (contains letters and possibly numbers/dots)
			if isValidInstanceType(instanceType) {
				specs = append(specs, InstanceSpec{Type: instanceType})
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

// isStdoutPiped returns true if stdout is being piped (not a terminal)
func isStdoutPiped() bool {
	stat, _ := os.Stdout.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// stderrPrintf prints to stderr (used when stdout is piped)
func stderrPrintf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
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

// RunGPUCreate executes the GPU create with retry logic
func RunGPUCreate(t *terminal.Terminal, gpuCreateStore GPUCreateStore, opts GPUCreateOptions) error {
	// Check if output is being piped (for chaining with brev shell/open)
	piped := isStdoutPiped()

	// Helper to print progress - uses stderr when piped so only instance name goes to stdout
	logf := func(format string, a ...interface{}) {
		if piped {
			fmt.Fprintf(os.Stderr, format, a...)
		} else {
			t.Vprintf(format, a...)
		}
	}

	user, err := gpuCreateStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	org, err := gpuCreateStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return breverrors.NewValidationError("no organization found")
	}

	// Fetch instance types with workspace groups to determine correct workspace group ID
	allInstanceTypes, err := gpuCreateStore.GetAllInstanceTypesWithWorkspaceGroups(org.ID)
	if err != nil {
		logf("Warning: could not fetch instance types with workspace groups: %s\n", err.Error())
		logf("Falling back to default workspace group\n")
		allInstanceTypes = nil
	}

	logf("Attempting to create %d instance(s) with %d parallel attempts\n", opts.Count, opts.Parallel)
	logf("Instance types to try: %s\n\n", formatInstanceSpecs(opts.InstanceTypes))

	// Track successful creations
	var successfulWorkspaces []*entity.Workspace
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Create a context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to coordinate attempts
	specsChan := make(chan InstanceSpec, len(opts.InstanceTypes))
	for _, spec := range opts.InstanceTypes {
		specsChan <- spec
	}
	close(specsChan)

	// Results channel
	resultsChan := make(chan CreateResult, len(opts.InstanceTypes))

	// Track instance index for naming (incremented only on successful creation)
	instanceIndex := 0

	// Start parallel workers
	workerCount := opts.Parallel
	if workerCount > len(opts.InstanceTypes) {
		workerCount = len(opts.InstanceTypes)
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for spec := range specsChan {
				// Check if we've already created enough
				mu.Lock()
				if len(successfulWorkspaces) >= opts.Count {
					mu.Unlock()
					return
				}
				// Get current index for naming (only increment on success)
				currentIndex := instanceIndex
				mu.Unlock()

				// Check context
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Determine instance name based on current successful count
				instanceName := opts.Name
				if opts.Count > 1 || currentIndex > 0 {
					instanceName = fmt.Sprintf("%s-%d", opts.Name, currentIndex+1)
				}

				logf("[Worker %d] Trying %s for instance '%s'...\n", workerID+1, spec.Type, instanceName)

				// Attempt to create the workspace
				workspace, err := createWorkspaceWithType(gpuCreateStore, org.ID, instanceName, spec.Type, spec.DiskGB, user, allInstanceTypes, opts.StartupScript)

				result := CreateResult{
					Workspace:    workspace,
					InstanceType: spec.Type,
					Error:        err,
				}

				if err != nil {
					errStr := err.Error()
					if piped {
						logf("[Worker %d] %s Failed: %s\n", workerID+1, spec.Type, errStr)
					} else {
						logf("[Worker %d] %s Failed: %s\n", workerID+1, t.Yellow(spec.Type), errStr)
					}

					// Check for fatal errors that should stop all workers
					if strings.Contains(errStr, "duplicate workspace") {
						logf("\nError: Workspace '%s' already exists. Use a different name or delete the existing workspace.\n", instanceName)
						cancel() // Stop all workers
						resultsChan <- result
						return
					}
				} else {
					if piped {
						logf("[Worker %d] %s Success! Created instance '%s'\n", workerID+1, spec.Type, instanceName)
					} else {
						logf("[Worker %d] %s Success! Created instance '%s'\n", workerID+1, t.Green(spec.Type), instanceName)
					}
					mu.Lock()
					successfulWorkspaces = append(successfulWorkspaces, workspace)
					instanceIndex++ // Only increment on success
					if len(successfulWorkspaces) >= opts.Count {
						cancel() // Signal other workers to stop
					}
					mu.Unlock()
				}

				resultsChan <- result
			}
		}(i)
	}

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for range resultsChan {
		// Just drain the channel
	}

	// Check if we created enough instances
	if len(successfulWorkspaces) < opts.Count {
		logf("\nWarning: Only created %d/%d instances\n", len(successfulWorkspaces), opts.Count)

		if len(successfulWorkspaces) > 0 {
			logf("Successfully created instances:\n")
			for _, ws := range successfulWorkspaces {
				logf("  - %s (ID: %s)\n", ws.Name, ws.ID)
			}
		}

		return breverrors.NewValidationError(fmt.Sprintf("could only create %d/%d instances", len(successfulWorkspaces), opts.Count))
	}

	// If we created more than needed, clean up extras
	if len(successfulWorkspaces) > opts.Count {
		extras := successfulWorkspaces[opts.Count:]
		logf("\nCleaning up %d extra instance(s)...\n", len(extras))

		for _, ws := range extras {
			logf("  Deleting %s...", ws.Name)
			_, err := gpuCreateStore.DeleteWorkspace(ws.ID)
			if err != nil {
				logf(" Failed\n")
			} else {
				logf(" Done\n")
			}
		}

		successfulWorkspaces = successfulWorkspaces[:opts.Count]
	}

	// Wait for instances to be ready (unless detached)
	if !opts.Detached {
		logf("\nWaiting for instance(s) to be ready...\n")
		logf("You can safely ctrl+c to exit\n")

		for _, ws := range successfulWorkspaces {
			err := pollUntilReady(t, ws.ID, gpuCreateStore, opts.Timeout, piped, logf)
			if err != nil {
				logf("  %s: Timeout waiting for ready state\n", ws.Name)
			}
		}
	}

	// If output is piped, just print instance name(s) for chaining with brev shell/open
	if piped {
		for _, ws := range successfulWorkspaces {
			fmt.Println(ws.Name)
		}
		return nil
	}

	// Print summary
	fmt.Print("\n")
	t.Vprint(t.Green(fmt.Sprintf("Successfully created %d instance(s)!\n\n", len(successfulWorkspaces))))

	for _, ws := range successfulWorkspaces {
		t.Vprintf("Instance: %s\n", t.Green(ws.Name))
		t.Vprintf("  ID: %s\n", ws.ID)
		t.Vprintf("  Type: %s\n", ws.InstanceType)
		displayConnectBreadCrumb(t, ws)
		fmt.Print("\n")
	}

	return nil
}

// createWorkspaceWithType creates a workspace with the specified instance type
func createWorkspaceWithType(gpuCreateStore GPUCreateStore, orgID, name, instanceType string, diskGB float64, user *entity.User, allInstanceTypes *gpusearch.AllInstanceTypesResponse, startupScript string) (*entity.Workspace, error) {
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	cwOptions := store.NewCreateWorkspacesOptions(clusterID, name)
	cwOptions.WithInstanceType(instanceType)
	cwOptions = resolveWorkspaceUserOptions(cwOptions, user)

	// Set disk size if specified (convert GB to Gi format)
	if diskGB > 0 {
		cwOptions.DiskStorage = fmt.Sprintf("%.0fGi", diskGB)
	}

	// Look up the workspace group ID for this instance type
	if allInstanceTypes != nil {
		workspaceGroupID := allInstanceTypes.GetWorkspaceGroupID(instanceType)
		if workspaceGroupID != "" {
			cwOptions.WorkspaceGroupID = workspaceGroupID
		}
	}

	// Set startup script if provided using VMBuild lifecycle script
	if startupScript != "" {
		cwOptions.VMBuild = &store.VMBuild{
			ForceJupyterInstall: true,
			LifeCycleScriptAttr: &store.LifeCycleScriptAttr{
				Script: startupScript,
			},
		}
	}

	workspace, err := gpuCreateStore.CreateWorkspace(orgID, cwOptions)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return workspace, nil
}

// resolveWorkspaceUserOptions sets workspace template and class based on user type
func resolveWorkspaceUserOptions(options *store.CreateWorkspacesOptions, user *entity.User) *store.CreateWorkspacesOptions {
	if options.WorkspaceTemplateID == "" {
		if featureflag.IsAdmin(user.GlobalUserType) {
			options.WorkspaceTemplateID = store.DevWorkspaceTemplateID
		} else {
			options.WorkspaceTemplateID = store.UserWorkspaceTemplateID
		}
	}
	if options.WorkspaceClassID == "" {
		if featureflag.IsAdmin(user.GlobalUserType) {
			options.WorkspaceClassID = store.DevWorkspaceClassID
		} else {
			options.WorkspaceClassID = store.UserWorkspaceClassID
		}
	}
	return options
}

// pollUntilReady waits for a workspace to reach the running state
func pollUntilReady(t *terminal.Terminal, wsID string, gpuCreateStore GPUCreateStore, timeout time.Duration, piped bool, logf func(string, ...interface{})) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ws, err := gpuCreateStore.GetWorkspace(wsID)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		if ws.Status == entity.Running {
			if piped {
				logf("  %s: Ready\n", ws.Name)
			} else {
				logf("  %s: %s\n", ws.Name, t.Green("Ready"))
			}
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
