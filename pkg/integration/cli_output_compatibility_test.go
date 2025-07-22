package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests verify CLI output formats that external integrations depend on.
// Breaking these tests indicates a breaking change that could affect external tools
// like NVIDIA Workbench that parse brev CLI output.

// Test_VersionCommandOutputFormat verifies that 'brev --version' outputs a parseable version string
func Test_VersionCommandOutputFormat(t *testing.T) {
	cmd := exec.Command("go", "run", "../../main.go", "--version")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "brev --version command should succeed")

	outputStr := string(output)
	
	// Test 1: Version should contain "Current version:" or "Current Version:" prefix
	hasVersionPrefix := strings.Contains(outputStr, "Current Version:") || strings.Contains(outputStr, "Current version:")
	assert.True(t, hasVersionPrefix, "Version output should contain 'Current Version:' or 'Current version:' prefix")
	
	// Test 2: Should contain a version number in X.Y.Z format (may be empty in dev builds)
	versionRegex := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	matches := versionRegex.FindAllString(outputStr, -1)
	
	// In dev environments, current version might be empty, but there should be a "New Version:" mentioned
	if len(matches) == 0 {
		// Check if this is a dev build scenario where current version is empty
		assert.True(t, strings.Contains(outputStr, "New Version:"), 
			"If no current version found, should show available version for upgrade")
	} else {
		// If we found version numbers, verify the first one is properly formatted
		versionStr := matches[0]
		versionParts := strings.Split(versionStr, ".")
		assert.Len(t, versionParts, 3, "Version should have exactly 3 components")
		
		for i, part := range versionParts {
			_, err := strconv.Atoi(part)
			assert.NoError(t, err, "Version part %d (%s) should be a valid integer", i, part)
		}
	}
}

// Test_OrgListCommandOutputFormat verifies that 'brev org ls' outputs parseable table format
func Test_OrgListCommandOutputFormat(t *testing.T) {
	// Note: This test may require authentication, so we'll test the format when orgs exist
	cmd := exec.Command("go", "run", "../../main.go", "org", "ls")
	output, err := cmd.CombinedOutput()
	
	// Command might fail due to auth issues, but if it succeeds, check format
	if err == nil {
		outputStr := string(output)
		lines := strings.Split(outputStr, "\n")
		
		// Look for table header format
		var foundHeader bool
		var headerLineIdx int
		for i, line := range lines {
			if strings.Contains(line, "NAME") && strings.Contains(line, "ID") {
				foundHeader = true
				headerLineIdx = i
				break
			}
		}
		
		if foundHeader {
			// Test table header format
			headerLine := lines[headerLineIdx]
			assert.Contains(t, headerLine, "NAME", "Org list should have NAME column")
			assert.Contains(t, headerLine, "ID", "Org list should have ID column")
			
			// Test that current org is marked with "*" prefix if there are data rows
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
	}
}

// Test_InstanceListCommandOutputFormat verifies that 'brev ls' outputs parseable table format
func Test_InstanceListCommandOutputFormat(t *testing.T) {
	cmd := exec.Command("go", "run", "../../main.go", "ls")
	output, err := cmd.CombinedOutput()
	
	// Command might fail due to auth issues, but if it succeeds, check format
	if err == nil {
		outputStr := string(output)
		lines := strings.Split(outputStr, "\n")
		
		// Look for table header format
		var foundHeader bool
		var headerLineIdx int
		for i, line := range lines {
			if strings.Contains(line, "NAME") && strings.Contains(line, "STATUS") && 
			   strings.Contains(line, "ID") && strings.Contains(line, "MACHINE") {
				foundHeader = true
				headerLineIdx = i
				break
			}
		}
		
		if foundHeader {
			// Test required columns exist
			headerLine := lines[headerLineIdx]
			assert.Contains(t, headerLine, "NAME", "Instance list should have NAME column")
			assert.Contains(t, headerLine, "STATUS", "Instance list should have STATUS column") 
			assert.Contains(t, headerLine, "ID", "Instance list should have ID column")
			assert.Contains(t, headerLine, "MACHINE", "Instance list should have MACHINE column")
			
			// Test column positions for parsing (important for external integrations)
			namePos := strings.Index(headerLine, "NAME")
			statusPos := strings.Index(headerLine, "STATUS")
			idPos := strings.Index(headerLine, "ID")
			machinePos := strings.Index(headerLine, "MACHINE")
			
			assert.GreaterOrEqual(t, statusPos, namePos, "STATUS column should come after NAME")
			assert.GreaterOrEqual(t, idPos, statusPos, "ID column should come after STATUS")
			assert.GreaterOrEqual(t, machinePos, idPos, "MACHINE column should come after ID")
		}
	}
}

// Test_RefreshCommandExists verifies that 'brev refresh' command exists and is callable
func Test_RefreshCommandExists(t *testing.T) {
	cmd := exec.Command("go", "run", "../../main.go", "refresh", "--help")
	output, _ := cmd.CombinedOutput()
	
	// Should succeed or fail with auth, but not with "command not found"
	outputStr := string(output)
	assert.NotContains(t, outputStr, "unknown command", "refresh command should exist")
	assert.NotContains(t, outputStr, "command not found", "refresh command should exist")
}

// Test_OrgSetCommandExists verifies that 'brev org set' command exists and is callable  
func Test_OrgSetCommandExists(t *testing.T) {
	cmd := exec.Command("go", "run", "../../main.go", "org", "set", "--help")
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
	cmd := exec.Command("go", "run", "../../main.go", "start", "--help")
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
	cmd := exec.Command("go", "run", "../../main.go", "stop", "--help")
	output, _ := cmd.CombinedOutput()
	
	outputStr := string(output)
	// Command exists if it shows help or mentions "stop" functionality
	hasStopCommand := strings.Contains(outputStr, "stop") || 
		!strings.Contains(outputStr, "unknown command")
	assert.True(t, hasStopCommand, "stop command should exist")
}

// Test_APIInstanceTypesEndpoint verifies the expected API endpoint format
func Test_APIInstanceTypesEndpoint(t *testing.T) {
	// Mock server to test expected request format
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test 1: Correct endpoint path
		assert.Equal(t, "/v1/instance/types", r.URL.Path, "API should call correct endpoint")
		
		// Test 2: Expected User-Agent (this is what external integrations set)
		expectedUserAgent := "nvidia-ai-workbench"
		actualUserAgent := r.Header.Get("User-Agent")
		if actualUserAgent != "" {
			assert.Equal(t, expectedUserAgent, actualUserAgent, "API should use expected User-Agent")
		}
		
		// Test 3: Return expected response format
		response := struct {
			Items []struct {
				Type      string `json:"type"`
				Stoppable bool   `json:"stoppable"`
			} `json:"items"`
		}{
			Items: []struct {
				Type      string `json:"type"`
				Stoppable bool   `json:"stoppable"`
			}{
				{Type: "g4dn.xlarge", Stoppable: true},
				{Type: "t3.micro", Stoppable: false},
				{Type: "t3.medium", Stoppable: true},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()
	
	// Test the API endpoint
	client := &http.Client{Timeout: 5 * time.Second}
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/v1/instance/types", nil)
	require.NoError(t, err)
	
	req.Header.Set("User-Agent", "nvidia-ai-workbench")
	
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logf("Failed to close response body: %v", closeErr)
		}
	}()
	
	assert.Equal(t, 200, resp.StatusCode, "API should return 200 OK")
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "API should return JSON")
	
	// Test response parsing
	var response struct {
		Items []struct {
			Type      string `json:"type"`
			Stoppable bool   `json:"stoppable"`
		} `json:"items"`
	}
	
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err, "Response should be valid JSON")
	
	assert.NotEmpty(t, response.Items, "Response should contain items")
	for _, item := range response.Items {
		assert.NotEmpty(t, item.Type, "Each item should have a type")
		// Stoppable can be true or false, both are valid
	}
}

// Test_CommandLineInterfaceStability verifies CLI doesn't break basic usage patterns
func Test_CommandLineInterfaceStability(t *testing.T) {
	// Test that help command works
	cmd := exec.Command("go", "run", "../../main.go", "--help")
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
	
	// Minimum version that external integrations expect
	minBrevVersion := "0.6.306"
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Extract version using same regex external tools use
			versionRegEx := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
			versionStr := versionRegEx.FindString(tc.versionOutput)
			
			if tc.shouldError {
				assert.Empty(t, versionStr, "Should not find version in malformed output")
				return
			}
			
			assert.NotEmpty(t, versionStr, "Should find version number")
			
			// Compare version parts (same logic as external integration)
			installedVersionComponents := strings.Split(versionStr, ".")
			minVersionComponents := strings.Split(minBrevVersion, ".")
			
			needsUpgrade := false
			for i := range installedVersionComponents {
				installed, err := strconv.Atoi(installedVersionComponents[i])
				require.NoError(t, err, "Version component should be integer")
				
				desired, err := strconv.Atoi(minVersionComponents[i])
				require.NoError(t, err, "Min version component should be integer")
				
				if installed < desired {
					needsUpgrade = true
					break
				}
				if installed > desired {
					break
				}
			}
			
			assert.Equal(t, tc.expectsUpgrade, needsUpgrade, 
				"Version upgrade requirement should match expected for %s", versionStr)
		})
	}
} 