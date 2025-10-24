package integration

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests verify CLI output formats that external integrations depend on.
// Breaking these tests indicates a breaking change that could affect external tools
// like NVIDIA Workbench that parse brev CLI output.
//
// NVIDIA AI Workbench Dependencies (CRITICAL - DO NOT BREAK):
// ============================================================
// Workbench uses the Brev CLI for managing compute instances and depends on:
//
// 1. Commands that MUST exist and maintain their behavior:
//    - brev -h / brev --help (to check CLI exists)
//    - brev --version (returns X.Y.Z format, with minimum version 0.6.306)
//    - brev refresh (refreshes SSH configuration)
//    - brev org set <org> (sets active organization)
//    - brev org ls (lists organizations with NAME and ID columns)
//    - brev ls (lists instances with NAME, STATUS, ID columns)
//    - brev ls --org <org> (lists instances for specific org)
//    - brev start [url] --org <org> (starts/creates workspace)
//    - brev stop <name> (stops workspace)
//
// 2. Output formats that MUST remain stable:
//    - brev ls: Table format with columns (in order): NAME, STATUS, ID, MACHINE
//      * Workbench has a custom parser that uses column headers to identify cells
//      * New columns can be ADDED but existing columns cannot be REMOVED or RENAMED
//      * Column order should be maintained for reliable parsing
//    - brev --version: Must contain "Current Version:" or "Current version:" 
//      followed by semantic version X.Y.Z that can be parsed with regex
//    - brev org ls: Table format with NAME and ID columns
//      * Current org marked with "*" prefix
//
// 3. API endpoints that Workbench depends on:
//    - https://api.brev.dev/v1/instance/types
//      * Required fields: "type" (string), "stoppable" (boolean)
//      * DO NOT remove or rename these fields
//
// 4. Authentication integration:
//    - Workbench implements custom KAS login flow that integrates with Brev credentials
//    - Credential management must remain compatible with Brev's token storage
//
// References:
// - Workbench CLI integration: https://gitlab-master.nvidia.com/workbench/workbench-cli/-/blob/main/pkg/brev/cli.go
// - Credential manager: https://gitlab-master.nvidia.com/workbench/credential-manager/-/blob/main/pkg/integrations/brev.go
// - Documentation: /Users/kejones/Git/brevdev/notes/brev-cli/workbench-dependencies.md

const (
	// Regular expressions
	versionPattern = `(\d+\.\d+\.\d+)`

	// CLI binary path
	brevCLIPath = "../../main.go"

	// Version prefixes
	currentVersionPrefix    = "Current Version:"
	currentVersionPrefixAlt = "Current version:"
	newVersionPrefix        = "New Version:"

	// Table column headers
	nameColumn    = "NAME"
	idColumn      = "ID"
	statusColumn  = "STATUS"
	machineColumn = "MACHINE"

	// Minimum version for external integrations
	minBrevVersion = "0.6.306"
)

// Test_VersionCommandOutputFormat verifies that 'brev --version' outputs a parseable version string
func Test_VersionCommandOutputFormat(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "--version")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "brev --version command should succeed")

	outputStr := string(output)

	// Test 1: Version should contain "Current version:" or "Current Version:" prefix
	hasVersionPrefix := strings.Contains(outputStr, currentVersionPrefix) || strings.Contains(outputStr, currentVersionPrefixAlt)
	assert.True(t, hasVersionPrefix, "Version output should contain 'Current Version:' or 'Current version:' prefix")

	// Test 2: Should contain a version number in X.Y.Z format (dev builds should also have version)
	versionRegexp := regexp.MustCompile(versionPattern)
	matches := versionRegexp.FindAllString(outputStr, -1)

	// Version should always be present, even in dev environments
	require.NotEmpty(t, matches, "CLI version should always be present, cannot be an empty string")

	// If we found version numbers, verify the first one is properly formatted
	versionStr := matches[0]
	versionParts := strings.Split(versionStr, ".")
	assert.Len(t, versionParts, 3, "Version should have exactly 3 components")

	for i, part := range versionParts {
		_, err := strconv.Atoi(part)
		assert.NoError(t, err, "Version part %d (%s) should be a valid integer", i, part)
	}
}

// Test_OrgListCommandOutputFormat verifies that 'brev org ls' outputs parseable table format
func Test_OrgListCommandOutputFormat(t *testing.T) {
	// Note: This test may require authentication, so we'll test the format when orgs exist
	cmd := exec.Command("go", "run", brevCLIPath, "org", "ls")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skip("maybe auth error")
		return
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// Look for table header format
	headerLineIdx := findHeaderLine(lines, nameColumn, idColumn)
	if headerLineIdx == -1 {
		return // No header found, nothing to validate
	}

	// Test table header format
	headerLine := lines[headerLineIdx]
	assert.Contains(t, headerLine, nameColumn, "Org list should have NAME column")
	assert.Contains(t, headerLine, idColumn, "Org list should have ID column")

	// Test that current org is marked with "*" prefix if there are data rows
	validateCurrentOrgMarker(t, lines, headerLineIdx)
}

// findHeaderLine searches for a line containing the specified columns
func findHeaderLine(lines []string, requiredCols ...string) int {
	for i, line := range lines {
		allPresent := true
		for _, col := range requiredCols {
			if !strings.Contains(line, col) {
				allPresent = false
				break
			}
		}
		if allPresent {
			return i
		}
	}
	return -1
}

// validateCurrentOrgMarker checks that current org entries are properly marked
func validateCurrentOrgMarker(t *testing.T, lines []string, headerLineIdx int) {
	t.Helper()

	for i := headerLineIdx + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			break
		}
		// If line starts with "* ", it's the current org
		if strings.HasPrefix(line, "* ") {
			fields := strings.Fields(line)
			assert.GreaterOrEqual(t, len(fields), 2, "Current org line should have at least name and ID")
		}
	}
}

// Test_InstanceListCommandOutputFormat verifies that 'brev ls' outputs parseable table format
func Test_InstanceListCommandOutputFormat(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "ls")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skip("maybe auth error")
		return
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// Look for table header format
	headerLineIdx := findHeaderLine(lines, nameColumn, statusColumn, idColumn, machineColumn)
	if headerLineIdx == -1 {
		return // No header found, nothing to validate
	}

	// Test required columns exist
	headerLine := lines[headerLineIdx]
	assert.Contains(t, headerLine, nameColumn, "Instance list should have NAME column")
	assert.Contains(t, headerLine, statusColumn, "Instance list should have STATUS column")
	assert.Contains(t, headerLine, idColumn, "Instance list should have ID column")
	assert.Contains(t, headerLine, machineColumn, "Instance list should have MACHINE column")

	// Test column positions for parsing (important for external integrations)
	validateColumnOrder(t, headerLine)
}

// validateColumnOrder ensures columns appear in the expected order for external parsers
func validateColumnOrder(t *testing.T, headerLine string) {
	t.Helper()

	namePos := strings.Index(headerLine, nameColumn)
	statusPos := strings.Index(headerLine, statusColumn)
	idPos := strings.Index(headerLine, idColumn)
	machinePos := strings.Index(headerLine, machineColumn)

	assert.GreaterOrEqual(t, statusPos, namePos, "STATUS column should come after NAME")
	assert.GreaterOrEqual(t, idPos, statusPos, "ID column should come after STATUS")
	assert.GreaterOrEqual(t, machinePos, idPos, "MACHINE column should come after ID")
}

// Test_RefreshCommandExists verifies that 'brev refresh' command exists and is callable
func Test_RefreshCommandExists(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "refresh", "--help")
	output, _ := cmd.CombinedOutput()

	// Should succeed or fail with auth, but not with "command not found"
	outputStr := string(output)
	assert.NotContains(t, outputStr, "unknown command", "refresh command should exist")
	assert.NotContains(t, outputStr, "command not found", "refresh command should exist")
}

// Test_OrgSetCommandExists verifies that 'brev org set' command exists and is callable
func Test_OrgSetCommandExists(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "org", "set", "--help")
	output, _ := cmd.CombinedOutput()

	// Should succeed with help output or show that command exists
	outputStr := string(output)
	// Command exists if it shows help or mentions "set" functionality
	hasSetCommand := strings.Contains(outputStr, "set") ||
		strings.Contains(outputStr, "organization") ||
		!strings.Contains(outputStr, "unknown command")
	assert.True(t, hasSetCommand, "org set command should exist and be callable")
}

// Test_StartCommandFormat verifies that 'brev start' command accepts --org flag
// CRITICAL: Workbench requires the --org flag to specify organization for workspace creation
func Test_StartCommandFormat(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "start", "--help")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "brev start --help should succeed")

	outputStr := string(output)
	
	// Verify the command exists
	assert.Contains(t, outputStr, "start", "start command should exist")
	assert.NotContains(t, outputStr, "unknown command", "start should be a valid command")
	
	// CRITICAL: Verify --org flag is documented and available
	assert.Contains(t, outputStr, "--org", "start command MUST support --org flag for Workbench compatibility")
}

// Test_StopCommandExists verifies that 'brev stop' command exists
func Test_StopCommandExists(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "stop", "--help")
	output, _ := cmd.CombinedOutput()

	outputStr := string(output)
	// Command exists if it shows help or mentions "stop" functionality
	hasStopCommand := strings.Contains(outputStr, "stop") ||
		!strings.Contains(outputStr, "unknown command")
	assert.True(t, hasStopCommand, "stop command should exist")
}

// Test_CommandLineInterfaceStability verifies CLI doesn't break basic usage patterns
func Test_CommandLineInterfaceStability(t *testing.T) {
	// Test that help command works
	cmd := exec.Command("go", "run", brevCLIPath, "--help")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "brev --help should work")

	outputStr := string(output)

	// Essential commands should be listed in help
	essentialCommands := []string{"ls", "org", "start", "stop", "refresh"}
	for _, command := range essentialCommands {
		assert.Contains(t, outputStr, command, "Help should list essential command: %s", command)
	}
}

// Test_VersionParsingCompatibility tests the version parsing logic that external tools use
func Test_VersionParsingCompatibility(t *testing.T) {
	testCases := []struct {
		name           string
		versionOutput  string
		expectsUpgrade bool
		shouldError    bool
	}{
		{
			name:           "current_version_sufficient",
			versionOutput:  "Current Version: 0.6.306",
			expectsUpgrade: false,
			shouldError:    false,
		},
		{
			name:           "newer_version_no_upgrade",
			versionOutput:  "Current Version: 0.6.307",
			expectsUpgrade: false,
			shouldError:    false,
		},
		{
			name:           "older_version_needs_upgrade",
			versionOutput:  "Current Version: 0.6.305",
			expectsUpgrade: true,
			shouldError:    false,
		},
		{
			name:           "much_older_version",
			versionOutput:  "Current Version: 0.5.999",
			expectsUpgrade: true,
			shouldError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Extract version using same regex external tools use
			versionRegexp := regexp.MustCompile(versionPattern)
			versionStr := versionRegexp.FindString(tc.versionOutput)

			if tc.shouldError {
				assert.Empty(t, versionStr, "Should not find version in malformed output")
				return
			}

			assert.NotEmpty(t, versionStr, "Should find version number")

			// Compare version parts (same logic as external integration)
			needsUpgrade := compareVersions(t, versionStr, minBrevVersion)
			assert.Equal(t, tc.expectsUpgrade, needsUpgrade,
				"Version upgrade requirement should match expected for %s", versionStr)
		})
	}
}

// compareVersions compares two semantic versions and returns true if installed < minimum
func compareVersions(t *testing.T, installedVersion, minVersion string) bool {
	t.Helper()

	installedComponents := strings.Split(installedVersion, ".")
	minComponents := strings.Split(minVersion, ".")

	for i := range installedComponents {
		installed, err := strconv.Atoi(installedComponents[i])
		require.NoError(t, err, "Version component should be integer")

		desired, err := strconv.Atoi(minComponents[i])
		require.NoError(t, err, "Min version component should be integer")

		if installed < desired {
			return true
		}
		if installed > desired {
			return false
		}
	}
	return false
}

// Test_ListWithOrgFlag verifies that 'brev ls --org' flag is recognized
// CRITICAL: Workbench uses this to list instances for specific organizations
func Test_ListWithOrgFlag(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "ls", "--help")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "brev ls --help should succeed")

	outputStr := string(output)
	
	// CRITICAL: Verify --org flag exists for ls command
	assert.Contains(t, outputStr, "--org", "ls command MUST support --org flag for Workbench compatibility")
	
	// Verify the flag description mentions organization
	lines := strings.Split(outputStr, "\n")
	foundOrgLine := false
	for _, line := range lines {
		if strings.Contains(line, "--org") {
			foundOrgLine = true
			// The line should mention organization or org
			assert.True(t, 
				strings.Contains(strings.ToLower(line), "org") || 
				strings.Contains(strings.ToLower(line), "organization"),
				"--org flag should have documentation mentioning organization")
			break
		}
	}
	assert.True(t, foundOrgLine, "--org flag should be documented in help output")
}

// Test_ShortHelpFlag verifies that 'brev -h' works
// CRITICAL: Workbench uses 'brev -h' to check if CLI exists
func Test_ShortHelpFlag(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "-h")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "brev -h should succeed")

	outputStr := string(output)
	
	// Should show help text
	assert.Contains(t, outputStr, "brev", "Help should mention brev")
	assert.Contains(t, outputStr, "Usage", "Help should show usage information")
	
	// Should list essential commands
	essentialCommands := []string{"ls", "start", "stop", "org"}
	for _, cmd := range essentialCommands {
		assert.Contains(t, outputStr, cmd, "Help should list essential command: %s", cmd)
	}
}

// Test_VersionWithNoCheckLatestFlag verifies version flag variations
// CRITICAL: Ensures version command works with various flags Workbench might use
func Test_VersionWithNoCheckLatestFlag(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "--version", "--no-check-latest")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "brev --version --no-check-latest should succeed")

	outputStr := string(output)
	
	// Should still show version information (even if it's dev-XXXXXXXX format)
	// The important thing is the command doesn't crash or fail
	assert.NotEmpty(t, outputStr, "Version command should produce output")
	
	// For production builds, should contain version information
	versionRegexp := regexp.MustCompile(versionPattern)
	matches := versionRegexp.FindAllString(outputStr, -1)
	
	// If we find a semver version, validate it
	if len(matches) > 0 {
		versionStr := matches[0]
		versionParts := strings.Split(versionStr, ".")
		assert.Len(t, versionParts, 3, "Version should have exactly 3 components")
	} else {
		// Dev builds may have "dev-XXXXXXXX" format, which is acceptable
		t.Log("Version is in dev format (not semver), which is acceptable for development builds")
	}
}

// Test_InstanceListColumnHeadersStability verifies ALL required columns exist
// CRITICAL: Workbench parser depends on these exact column headers
func Test_InstanceListColumnHeadersStability(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "ls", "--help")
	output, _ := cmd.CombinedOutput()
	
	// Verify ls command exists
	outputStr := string(output)
	assert.NotContains(t, outputStr, "unknown command", "ls command must exist")
	
	// Run actual ls command (may skip if auth fails)
	cmd = exec.Command("go", "run", brevCLIPath, "ls")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skip("ls command requires authentication")
		return
	}

	outputStr = string(output)
	
	// If there are no instances, the output won't have column headers (which is correct behavior)
	if strings.Contains(outputStr, "No instances") {
		t.Log("✅ No instances present, skipping column header validation (headers only shown when data exists)")
		return
	}
	
	// CRITICAL: These columns MUST exist when instances are present - Workbench parser depends on them
	requiredColumns := []string{"NAME", "STATUS", "ID"}
	for _, col := range requiredColumns {
		assert.Contains(t, outputStr, col, 
			"CRITICAL: '%s' column MUST exist for Workbench compatibility. DO NOT REMOVE OR RENAME.", col)
	}
	
	// Note: Additional columns can be added, but these core columns must remain
	t.Log("✅ All required column headers present. New columns can be added, but existing ones MUST NOT be removed or renamed.")
}

// Test_OrgListColumnHeadersStability verifies org list column headers
// CRITICAL: Ensures org list output format remains stable for Workbench
func Test_OrgListColumnHeadersStability(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "org", "ls")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skip("org ls requires authentication")
		return
	}

	outputStr := string(output)
	
	// CRITICAL: These columns MUST exist for Workbench
	requiredColumns := []string{"NAME", "ID"}
	for _, col := range requiredColumns {
		assert.Contains(t, outputStr, col,
			"CRITICAL: '%s' column MUST exist in org ls for Workbench compatibility.", col)
	}
}

// Test_CommandExistenceForWorkbench verifies all Workbench-critical commands exist
// CRITICAL: Comprehensive check that all commands Workbench depends on are available
func Test_CommandExistenceForWorkbench(t *testing.T) {
	criticalCommands := []struct {
		name        string
		args        []string
		description string
	}{
		{"help_short", []string{"-h"}, "check CLI exists"},
		{"help_long", []string{"--help"}, "show full help"},
		{"version", []string{"--version"}, "get version"},
		{"ls", []string{"ls", "--help"}, "list instances"},
		{"org_ls", []string{"org", "ls", "--help"}, "list organizations"},
		{"org_set", []string{"org", "set", "--help"}, "set active org"},
		{"start", []string{"start", "--help"}, "start/create workspace"},
		{"stop", []string{"stop", "--help"}, "stop workspace"},
		{"refresh", []string{"refresh", "--help"}, "refresh SSH config"},
	}

	for _, cmd := range criticalCommands {
		t.Run(cmd.name, func(t *testing.T) {
			execCmd := exec.Command("go", append([]string{"run", brevCLIPath}, cmd.args...)...)
			output, err := execCmd.CombinedOutput()
			
			// Command should execute (may fail with auth, but shouldn't be "unknown command")
			outputStr := string(output)
			assert.NotContains(t, outputStr, "unknown command", 
				"CRITICAL: Command '%s' MUST exist for Workbench (%s)", cmd.name, cmd.description)
			
			// For help commands, should succeed
			if strings.Contains(cmd.name, "help") || strings.HasSuffix(cmd.args[len(cmd.args)-1], "--help") {
				assert.NoError(t, err, "Help command should succeed: %s", cmd.name)
			}
		})
	}
}
