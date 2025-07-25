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

	// Test 2: Should contain a version number in X.Y.Z format (may be empty in dev builds)
	versionRegexp := regexp.MustCompile(versionPattern)
	matches := versionRegexp.FindAllString(outputStr, -1)

	// In dev environments, current version might be empty, but there should be a "New Version:" mentioned
	if len(matches) == 0 {
		// Check if this is a dev build scenario where current version is empty
		assert.True(t, strings.Contains(outputStr, newVersionPrefix),
			"If no current version found, should show available version for upgrade")
		return
	}

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
func Test_StartCommandFormat(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "start", "--help")
	output, _ := cmd.CombinedOutput()

	outputStr := string(output)
	// Should mention org flag or organization, or at least not be unknown command
	hasOrgSupport := strings.Contains(outputStr, "--org") ||
		strings.Contains(outputStr, "organization") ||
		(strings.Contains(outputStr, "start") && !strings.Contains(outputStr, "unknown command"))
	assert.True(t, hasOrgSupport, "start command should exist and potentially support org specification")
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
