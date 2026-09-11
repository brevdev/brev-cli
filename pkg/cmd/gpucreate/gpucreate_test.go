package gpucreate

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockGPUCreateStore is a mock implementation of GPUCreateStore for testing
type MockGPUCreateStore struct {
	User                      *entity.User
	Org                       *entity.Organization
	Workspaces                map[string]*entity.Workspace
	CreateError               error
	CreateErrorTypes          map[string]error // Errors for specific instance types
	DeleteError               error
	CreatedOptions            []*store.CreateWorkspacesOptions
	CreatedWorkspaces         []*entity.Workspace
	DeletedWorkspaceIDs       []string
	FetchedLifeCycleScriptIDs []string
	AllInstanceTypes          *gpusearch.AllInstanceTypesResponse
	InstanceTypesIncludeCPU   []bool
}

func NewMockGPUCreateStore() *MockGPUCreateStore {
	return &MockGPUCreateStore{
		User: &entity.User{
			ID:             "user-123",
			GlobalUserType: "Standard",
		},
		Org: &entity.Organization{
			ID:   "org-123",
			Name: "test-org",
		},
		Workspaces:          make(map[string]*entity.Workspace),
		CreateErrorTypes:    make(map[string]error),
		CreatedWorkspaces:   []*entity.Workspace{},
		DeletedWorkspaceIDs: []string{},
	}
}

func (m *MockGPUCreateStore) GetCurrentUser() (*entity.User, error) {
	return m.User, nil
}

func (m *MockGPUCreateStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return nil, nil
}

func (m *MockGPUCreateStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return m.Org, nil
}

func (m *MockGPUCreateStore) GetWorkspace(workspaceID string) (*entity.Workspace, error) {
	if ws, ok := m.Workspaces[workspaceID]; ok {
		return ws, nil
	}
	return &entity.Workspace{
		ID:     workspaceID,
		Status: entity.Running,
	}, nil
}

func (m *MockGPUCreateStore) CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error) {
	// Check for type-specific errors first
	if err, ok := m.CreateErrorTypes[options.InstanceType]; ok {
		return nil, err
	}

	if m.CreateError != nil {
		return nil, m.CreateError
	}

	ws := &entity.Workspace{
		ID:           "ws-" + options.Name,
		Name:         options.Name,
		InstanceType: options.InstanceType,
		Status:       entity.Running,
	}
	m.Workspaces[ws.ID] = ws
	m.CreatedOptions = append(m.CreatedOptions, options)
	m.CreatedWorkspaces = append(m.CreatedWorkspaces, ws)
	return ws, nil
}

func (m *MockGPUCreateStore) DeleteWorkspace(workspaceID string) (*entity.Workspace, error) {
	if m.DeleteError != nil {
		return nil, m.DeleteError
	}

	m.DeletedWorkspaceIDs = append(m.DeletedWorkspaceIDs, workspaceID)
	ws := m.Workspaces[workspaceID]
	delete(m.Workspaces, workspaceID)
	return ws, nil
}

func (m *MockGPUCreateStore) GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error) {
	return []entity.Workspace{}, nil
}

func (m *MockGPUCreateStore) GetAllInstanceTypesWithCloudCreds(orgID string) (*gpusearch.AllInstanceTypesResponse, error) {
	return m.AllInstanceTypes, nil
}

func (m *MockGPUCreateStore) GetLaunchable(launchableID string) (*store.LaunchableResponse, error) {
	return &store.LaunchableResponse{
		ID:   launchableID,
		Name: "test-launchable",
	}, nil
}

func (m *MockGPUCreateStore) GetLaunchableLifeCycleScript(launchableID, scriptID string) (*store.LifeCycleScriptResponse, error) {
	m.FetchedLifeCycleScriptIDs = append(m.FetchedLifeCycleScriptIDs, scriptID)
	return &store.LifeCycleScriptResponse{
		Attrs: &store.LifeCycleScriptAttr{ID: scriptID, Script: "echo mock-script"},
	}, nil
}

func (m *MockGPUCreateStore) RedeemCouponCode(organizationID string, code string) (*store.RedeemCouponCodeResponse, error) {
	return &store.RedeemCouponCodeResponse{}, nil
}

func (m *MockGPUCreateStore) GetInstanceTypes(includeCPU bool) (*gpusearch.InstanceTypesResponse, error) {
	m.InstanceTypesIncludeCPU = append(m.InstanceTypesIncludeCPU, includeCPU)
	items := []gpusearch.InstanceType{
		{
			Type:               "g5.xlarge",
			Location:           "us-east-1",
			SubLocation:        "us-east-1a",
			AvailableLocations: []string{"us-east-1", "us-west-2"},
			SupportedGPUs: []gpusearch.GPU{
				{Count: 1, Name: "A10G", Manufacturer: "NVIDIA", Memory: "24GiB"},
			},
			SupportedStorage: []gpusearch.Storage{
				{Size: "500GiB"},
			},
			Memory:              "16GiB",
			VCPU:                4,
			BasePrice:           gpusearch.BasePrice{Currency: "USD", Amount: "1.006"},
			EstimatedDeployTime: "5m0s",
		},
	}
	if includeCPU {
		items = append(items, gpusearch.InstanceType{
			Type:               "n2d-standard-2",
			Location:           "asia-south1",
			SubLocation:        "asia-south1-a",
			AvailableLocations: []string{"asia-south1", "us-west2"},
			SupportedStorage: []gpusearch.Storage{
				{MinSize: "10GiB", MaxSize: "65536GiB"},
			},
			Memory:              "8GiB",
			VCPU:                2,
			BasePrice:           gpusearch.BasePrice{Currency: "USD", Amount: "0.097"},
			EstimatedDeployTime: "5m0s",
		})
	}

	// Return a default set of instance types for testing
	return &gpusearch.InstanceTypesResponse{
		Items: items,
	}, nil
}

func executeCreateAndCaptureOutput(t *testing.T, mock *MockGPUCreateStore, args ...string) string {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)
	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
	}()

	cmd := NewCmdGPUCreate(terminal.New(), mock)
	cmd.SetArgs(args)
	executeErr := cmd.Execute()
	require.NoError(t, executeErr)
	require.NoError(t, stdoutWriter.Close())
	require.NoError(t, stderrWriter.Close())
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	stdout, readErr := io.ReadAll(stdoutReader)
	require.NoError(t, readErr)
	stderr, readErr := io.ReadAll(stderrReader)
	require.NoError(t, readErr)
	return string(stdout) + string(stderr)
}

func TestIsValidInstanceType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid AWS instance type", "g5.xlarge", true},
		{"Valid AWS large instance", "p4d.24xlarge", true},
		{"Valid GCP instance type", "n1-highmem-4:nvidia-tesla-t4:1", true},
		{"Single letter", "a", false},
		{"No numbers", "xlarge", false},
		{"No letters", "12345", false},
		{"Empty string", "", false},
		{"Single character", "1", false},
		{"Valid with colon", "g5:xlarge", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidInstanceType(tt.input)
			assert.Equal(t, tt.expected, result, "Validation failed for %s", tt.input)
		})
	}
}

func TestParseLaunchableID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{"Empty string", "", "", false},
		{"Raw ID", "env-2jeVokEK44iJZzleTF8yKjt3hh7", "env-2jeVokEK44iJZzleTF8yKjt3hh7", false},
		{"Console URL with query param", "https://console.brev.dev/launchable/deploy?launchableID=env-abc123", "env-abc123", false},
		{"Console URL with extra params", "https://console.brev.dev/launchable/deploy?userID=u1&launchableID=env-abc123&name=test", "env-abc123", false},
		{"URL with env- in path", "https://console.brev.dev/launchables/env-abc123", "env-abc123", false},
		{"URL without launchableID param", "https://console.brev.dev/launchable/deploy", "", true},
		{"Non-env ID", "some-other-id", "some-other-id", false},
		{"ID with path chars", "env-abc/../../etc", "", true},
		{"ID with query chars", "env-abc?foo=bar", "", true},
		{"URL query param traversal", "https://console.brev.dev/launchable/deploy?launchableID=env-abc/../../other", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLaunchableID(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseLaunchableParameterValues(t *testing.T) {
	values, err := parseLaunchableParameterValues([]string{"MODEL=llama=3", "PORT=8080"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"MODEL": "llama=3", "PORT": "8080"}, values)

	_, err = parseLaunchableParameterValues([]string{"MISSING_SEPARATOR"})
	assert.ErrorContains(t, err, "expected NAME=VALUE")

	_, err = parseLaunchableParameterValues([]string{"MODEL=a", "MODEL=b"})
	assert.ErrorContains(t, err, "provided more than once")
}

func TestResolveLaunchableParameterBindings(t *testing.T) {
	info := &store.LaunchableResponse{
		BuildRequest: store.LaunchableBuildRequest{
			Parameters: []store.Parameter{
				{Name: "MODEL", Required: true, Choice: &store.ChoiceParameter{
					Choices: []string{"llama", "nemotron"},
				}},
				{Name: "REGION", Required: true, Choice: &store.ChoiceParameter{
					Choices: []string{"us", "eu"},
				}},
				{Name: "TAG", Text: &store.TextParameter{DefaultValue: "latest"}},
				{Name: "OPTIONAL", Text: &store.TextParameter{}},
			},
		},
	}

	bindings, err := resolveLaunchableParameterBindings(info, map[string]string{"MODEL": "nemotron", "REGION": "us"})
	require.NoError(t, err)
	assert.Equal(t, []store.ParameterBinding{
		{Name: "MODEL", Value: "nemotron"},
		{Name: "REGION", Value: "us"},
		{Name: "TAG", Value: "latest"},
	}, bindings)

	_, err = resolveLaunchableParameterBindings(info, map[string]string{
		"MODEL":  "unknown",
		"REGION": "asia",
		"TYPO":   "value",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown parameter "TYPO"`)
	assert.ErrorContains(t, err, `invalid value "unknown" for "MODEL"`)
	assert.ErrorContains(t, err, `invalid value "asia" for "REGION"`)

	_, err = resolveLaunchableParameterBindings(info, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, `missing required parameter "MODEL"; provide --param MODEL=VALUE`)
	assert.ErrorContains(t, err, `missing required parameter "REGION"; provide --param REGION=VALUE`)
}

func TestLaunchableParameterDisplayLines(t *testing.T) {
	parameters := []store.Parameter{
		{
			Name:        "B",
			Description: "Optional parameter",
			Text:        &store.TextParameter{},
		},
		{
			Name:        "A",
			Description: "Text parameter",
			Required:    true,
			Text:        &store.TextParameter{DefaultValue: "default-a"},
		},
		{
			Name:        "C",
			Description: "Choice parameter",
			Required:    true,
			Choice: &store.ChoiceParameter{
				Choices:      []string{"one", "two"},
				DefaultValue: "one",
			},
		},
	}

	assert.Equal(t, []string{
		"Required Parameters:",
		"    A    Text parameter (default: default-a)",
		"    C    Choice parameter (allowed: one, two; default: one)",
		"Optional Parameters:",
		"    B    Optional parameter",
	}, launchableParameterDisplayLines(parameters))
	assert.Nil(t, launchableParameterDisplayLines(nil))
}

func TestApplyLaunchableConfig(t *testing.T) { //nolint:funlen // test
	t.Run("populates all fields from launchable", func(t *testing.T) {
		cwOptions := &store.CreateWorkspacesOptions{
			PortMappings: map[string]string{},
		}
		info := &store.LaunchableResponse{
			ID:              "env-abc123",
			Name:            "Test Launchable",
			CreatedByUserID: "user-1",
			CreatedByOrgID:  "org-1",
			CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
				CloudCredID:  "GCP",
				InstanceType: "n2-standard-4",
				Storage:      "256",
				Location:     "us-west1",
				SubLocation:  "us-west1-b",
				ImageID:      "projects/example/global/images/custom-image",
				FirewallRules: []store.CreateFirewallRule{
					{Port: "8080", AllowedIPs: "all"},
					{Port: "9000-9100", AllowedIPs: "all"},
				},
			},
			BuildRequest: store.LaunchableBuildRequest{
				VMBuild: &store.VMBuild{
					ForceJupyterInstall: false,
					LifeCycleScriptAttr: &store.LifeCycleScriptAttr{
						Script: "#!/bin/bash\necho hello",
						ID:     "ls-abc",
					},
				},
				Ports: []store.LaunchablePort{
					{Name: "Code-Server", Port: "13337"},
					{Name: "OpenClaw", Port: "18789"},
				},
			},
			File: &store.LaunchableFile{
				URL:  "https://github.com/NVIDIA/NemoClaw",
				Path: "./",
			},
		}

		applyLaunchableConfig(cwOptions, "env-abc123", info, nil)

		// Cloud credential from launchable input.
		assert.Equal(t, "GCP", cwOptions.CloudCredID)
		// Location / sub-location
		assert.Equal(t, "us-west1", cwOptions.Location)
		assert.Equal(t, "us-west1-b", cwOptions.SubLocation)
		assert.Equal(t, "projects/example/global/images/custom-image", cwOptions.BaseImage)
		// Storage with Gi suffix
		assert.Equal(t, "256Gi", cwOptions.DiskStorage)
		// Build config
		assert.NotNil(t, cwOptions.VMBuild)
		assert.False(t, cwOptions.VMBuild.ForceJupyterInstall)
		assert.Equal(t, "#!/bin/bash\necho hello", cwOptions.VMBuild.LifeCycleScriptAttr.Script)
		assert.Equal(t, "ls-abc", cwOptions.VMBuild.LifeCycleScriptAttr.ID)
		// Port mappings
		assert.Equal(t, map[string]string{"Code-Server": "13337", "OpenClaw": "18789"}, cwOptions.PortMappings)
		assert.Equal(t, []store.CreateFirewallRule{
			{Port: "8080", AllowedIPs: "all"},
			{Port: "9000-9100", AllowedIPs: "all"},
		}, cwOptions.FirewallRules)
		// Files
		assert.NotNil(t, cwOptions.Files)
		// LaunchableConfig
		assert.Equal(t, "env-abc123", cwOptions.LaunchableConfig.ID)
		// Labels
		labels, ok := cwOptions.Labels.(map[string]string)
		assert.True(t, ok)
		assert.Equal(t, "env-abc123", labels["launchableId"])
		assert.Equal(t, "n2-standard-4", labels["launchableInstanceType"])
		assert.Equal(t, "GCP", labels["cloudCredId"])
		assert.NotContains(t, labels, "workspaceGroupId")
		assert.Equal(t, "user-1", labels["launchableCreatedByUserId"])
		assert.Equal(t, "org-1", labels["launchableCreatedByOrgId"])
	})

	t.Run("preserves existing cloud credential", func(t *testing.T) {
		cwOptions := &store.CreateWorkspacesOptions{
			CloudCredID: "existing-cloud-cred",
		}
		info := &store.LaunchableResponse{
			CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
				CloudCredID:  "GCP",
				InstanceType: "n2-standard-4",
			},
		}

		applyLaunchableConfig(cwOptions, "env-abc", info, nil)

		assert.Equal(t, "existing-cloud-cred", cwOptions.CloudCredID)
	})

	t.Run("storage already has Gi suffix", func(t *testing.T) {
		cwOptions := &store.CreateWorkspacesOptions{}
		info := &store.LaunchableResponse{
			CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
				Storage: "256Gi",
			},
		}

		applyLaunchableConfig(cwOptions, "env-abc", info, nil)

		assert.Equal(t, "256Gi", cwOptions.DiskStorage)
	})

	t.Run("storage bare number gets Gi suffix", func(t *testing.T) {
		cwOptions := &store.CreateWorkspacesOptions{}
		info := &store.LaunchableResponse{
			CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
				Storage: "256",
			},
		}

		applyLaunchableConfig(cwOptions, "env-abc", info, nil)

		assert.Equal(t, "256Gi", cwOptions.DiskStorage)
	})

	t.Run("storage with other suffix passes through", func(t *testing.T) {
		cwOptions := &store.CreateWorkspacesOptions{}
		info := &store.LaunchableResponse{
			CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
				Storage: "100G",
			},
		}

		applyLaunchableConfig(cwOptions, "env-abc", info, nil)

		assert.Equal(t, "100G", cwOptions.DiskStorage)
	})

	t.Run("container build clears VMBuild", func(t *testing.T) {
		cwOptions := &store.CreateWorkspacesOptions{
			VMBuild: &store.VMBuild{ForceJupyterInstall: true},
		}
		info := &store.LaunchableResponse{
			BuildRequest: store.LaunchableBuildRequest{
				CustomContainer: &store.CustomContainer{
					ContainerURL: "nvcr.io/nvidia/test:latest",
				},
			},
		}

		applyLaunchableConfig(cwOptions, "env-abc", info, nil)

		assert.Nil(t, cwOptions.VMBuild)
		assert.Equal(t, "nvcr.io/nvidia/test:latest", cwOptions.CustomContainer.ContainerURL)
	})

	t.Run("docker compose file URL takes precedence over hydrated YAML", func(t *testing.T) {
		cwOptions := &store.CreateWorkspacesOptions{
			VMBuild: &store.VMBuild{ForceJupyterInstall: true},
		}
		dockerCompose := &store.DockerCompose{
			FileURL:    "https://example.com/compose.yaml",
			YamlString: "services:\n  app:\n    image: example/app",
		}
		info := &store.LaunchableResponse{
			BuildRequest: store.LaunchableBuildRequest{
				DockerCompose: dockerCompose,
			},
		}

		applyLaunchableConfig(cwOptions, "env-abc", info, nil)

		assert.Nil(t, cwOptions.VMBuild)
		assert.Equal(t, dockerCompose.FileURL, cwOptions.DockerCompose.FileURL)
		assert.Empty(t, cwOptions.DockerCompose.YamlString)
		assert.NotEmpty(t, dockerCompose.YamlString, "launchable response should not be mutated")
	})

	t.Run("substitutes public IP for user-ip firewall rules", func(t *testing.T) {
		orig := publicIPLookup
		publicIPLookup = func() (string, error) { return "203.0.113.7", nil }
		defer func() { publicIPLookup = orig }()

		cwOptions := &store.CreateWorkspacesOptions{}
		info := &store.LaunchableResponse{
			CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
				InstanceType: "n2-standard-4",
				FirewallRules: []store.CreateFirewallRule{
					{Port: "22", AllowedIPs: "user-ip"},
					{Port: "443", AllowedIPs: "all"},
				},
			},
		}

		applyLaunchableConfig(cwOptions, "env-abc", info, nil)

		assert.Equal(t, []store.CreateFirewallRule{
			{Port: "22", AllowedIPs: "user-ip", ClientIPs: []string{"203.0.113.7/32"}},
			{Port: "443", AllowedIPs: "all"},
		}, cwOptions.FirewallRules)
	})

	t.Run("preserves rule when public IP lookup fails", func(t *testing.T) {
		rules := []store.CreateFirewallRule{
			{Port: "22", AllowedIPs: "user-ip"},
		}
		resolved := resolveFirewallRulesClientIP(rules, func() (string, error) {
			return "", assert.AnError
		})
		assert.Equal(t, []store.CreateFirewallRule{
			{Port: "22", AllowedIPs: "user-ip"},
		}, resolved)
	})

	t.Run("does not call lookup when no user-ip rules", func(t *testing.T) {
		called := false
		rules := []store.CreateFirewallRule{
			{Port: "8080", AllowedIPs: "all"},
		}
		resolveFirewallRulesClientIP(rules, func() (string, error) {
			called = true
			return "1.2.3.4", nil
		})
		assert.False(t, called, "lookup should be skipped when no user-ip rules exist")
	})

	t.Run("respects pre-existing ClientIPs on user-ip rule", func(t *testing.T) {
		rules := []store.CreateFirewallRule{
			{Port: "22", AllowedIPs: "user-ip", ClientIPs: []string{"198.51.100.5/32"}},
		}
		resolved := resolveFirewallRulesClientIP(rules, func() (string, error) {
			return "203.0.113.7", nil
		})
		assert.Equal(t, []string{"198.51.100.5/32"}, resolved[0].ClientIPs)
	})

	t.Run("uses /128 for IPv6 public IPs", func(t *testing.T) {
		rules := []store.CreateFirewallRule{
			{Port: "22", AllowedIPs: "user-ip"},
		}
		resolved := resolveFirewallRulesClientIP(rules, func() (string, error) {
			return "2403:2500:4000:0000:0000:0000:0000:090a", nil
		})
		assert.Equal(t, []string{"2403:2500:4000::90a/128"}, resolved[0].ClientIPs)
	})

	t.Run("canonicalizes IPv4-mapped IPv6 to bare IPv4 with /32", func(t *testing.T) {
		rules := []store.CreateFirewallRule{
			{Port: "22", AllowedIPs: "user-ip"},
		}
		resolved := resolveFirewallRulesClientIP(rules, func() (string, error) {
			return "::ffff:203.0.113.7", nil
		})
		assert.Equal(t, []string{"203.0.113.7/32"}, resolved[0].ClientIPs)
	})

	t.Run("skips rule when lookup returns garbage", func(t *testing.T) {
		rules := []store.CreateFirewallRule{
			{Port: "22", AllowedIPs: "user-ip"},
		}
		resolved := resolveFirewallRulesClientIP(rules, func() (string, error) {
			return "<html>captive portal</html>", nil
		})
		assert.Equal(t, []store.CreateFirewallRule{
			{Port: "22", AllowedIPs: "user-ip"},
		}, resolved)
	})

	t.Run("merges with existing labels", func(t *testing.T) {
		cwOptions := &store.CreateWorkspacesOptions{
			Labels: map[string]string{"existingKey": "existingValue"},
		}
		info := &store.LaunchableResponse{
			CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
				InstanceType: "n2-standard-4",
			},
			CreatedByUserID: "user-1",
			CreatedByOrgID:  "org-1",
		}

		applyLaunchableConfig(cwOptions, "env-abc", info, nil)

		labels, ok := cwOptions.Labels.(map[string]string)
		assert.True(t, ok)
		assert.Equal(t, "existingValue", labels["existingKey"])
		assert.Equal(t, "env-abc", labels["launchableId"])
	})
}

func TestToHostCIDR(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"IPv4", "203.0.113.7", "203.0.113.7/32"},
		{"IPv4 with surrounding whitespace", "  203.0.113.7\n", "203.0.113.7/32"},
		{"IPv4-mapped IPv6 canonicalizes to IPv4", "::ffff:203.0.113.7", "203.0.113.7/32"},
		{"IPv6 short form", "2403:2500:4000::90a", "2403:2500:4000::90a/128"},
		{"IPv6 long form canonicalizes", "2403:2500:4000:0000:0000:0000:0000:090a", "2403:2500:4000::90a/128"},
		{"IPv6 loopback", "::1", "::1/128"},
		{"empty", "", ""},
		{"not an IP", "not-an-ip", ""},
		{"captive portal HTML", "<html>", ""},
		{"IPv4 with port appended", "203.0.113.7:443", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, toHostCIDR(tt.in))
		})
	}
}

// TestLaunchableJSONWireFormat guards against silent regressions in the JSON
// tags on CreateWorkspacesOptions — the original bug was that fields the
// server expected (firewallRules, subLocation) were missing from the request
// body. Asserting on the marshaled bytes catches typos like "firewall_rules"
// or a dropped tag that struct-field assertions can't.
func TestLaunchableJSONWireFormat(t *testing.T) {
	cwOptions := &store.CreateWorkspacesOptions{}
	info := &store.LaunchableResponse{
		ID: "env-abc",
		CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
			InstanceType: "n2-standard-4",
			Location:     "us-west1",
			SubLocation:  "us-west1-b",
			ImageID:      "projects/example/global/images/custom-image",
			FirewallRules: []store.CreateFirewallRule{
				{Port: "8080", AllowedIPs: "all"},
				{Port: "22", AllowedIPs: "user-ip", ClientIPs: []string{"203.0.113.7/32"}},
			},
		},
	}

	applyLaunchableConfig(cwOptions, "env-abc", info, []store.ParameterBinding{
		{Name: "MODEL", Value: "llama"},
	})

	body, err := json.Marshal(cwOptions)
	assert.NoError(t, err)
	s := string(body)

	assert.Contains(t, s, `"subLocation":"us-west1-b"`)
	assert.Contains(t, s, `"baseImage":"projects/example/global/images/custom-image"`)
	assert.Contains(t, s, `"firewallRules":`)
	assert.Contains(t, s, `"port":"8080"`)
	assert.Contains(t, s, `"port":"22"`)
	assert.Contains(t, s, `"allowedIPs":"all"`)
	assert.Contains(t, s, `"allowedIPs":"user-ip"`)
	assert.Contains(t, s, `"clientIPs":["203.0.113.7/32"]`)
	assert.Contains(t, s, `"launchableConfig":{"id":"env-abc","parameterBindings":[{"name":"MODEL","value":"llama"}]}`)
}

func TestFetchPublicIP(t *testing.T) {
	client := &http.Client{Timeout: 2 * time.Second}

	t.Run("returns IP for plain text body", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("203.0.113.7\n"))
		}))
		defer s.Close()

		ip, err := fetchPublicIP(client, s.URL)
		assert.NoError(t, err)
		assert.Equal(t, "203.0.113.7", ip)
	})

	t.Run("rejects non-200 status", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer s.Close()

		_, err := fetchPublicIP(client, s.URL)
		assert.Error(t, err)
	})

	t.Run("rejects non-IP body (captive portal HTML)", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("<html>login required</html>"))
		}))
		defer s.Close()

		_, err := fetchPublicIP(client, s.URL)
		assert.Error(t, err)
	})
}

func TestResolvePublicIPFallback(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer failing.Close()

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("198.51.100.42"))
	}))
	defer ok.Close()

	t.Run("falls through failing endpoints until one succeeds", func(t *testing.T) {
		orig := publicIPEndpoints
		publicIPEndpoints = []string{failing.URL, failing.URL, ok.URL}
		defer func() { publicIPEndpoints = orig }()

		ip, err := resolvePublicIP()
		assert.NoError(t, err)
		assert.Equal(t, "198.51.100.42", ip)
	})

	t.Run("returns error when all endpoints fail", func(t *testing.T) {
		orig := publicIPEndpoints
		publicIPEndpoints = []string{failing.URL, failing.URL}
		defer func() { publicIPEndpoints = orig }()

		_, err := resolvePublicIP()
		assert.Error(t, err)
	})

	t.Run("short-circuits on first success", func(t *testing.T) {
		hits := 0
		counted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits++
			_, _ = w.Write([]byte("203.0.113.7"))
		}))
		defer counted.Close()

		orig := publicIPEndpoints
		publicIPEndpoints = []string{counted.URL, failing.URL, ok.URL}
		defer func() { publicIPEndpoints = orig }()

		ip, err := resolvePublicIP()
		assert.NoError(t, err)
		assert.Equal(t, "203.0.113.7", ip)
		assert.Equal(t, 1, hits, "later endpoints must not be hit after success")
	})
}

func TestParseInstanceTypesFromFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Single type", "g5.xlarge", []string{"g5.xlarge"}},
		{"Multiple types comma separated", "g5.xlarge,g5.2xlarge,p3.2xlarge", []string{"g5.xlarge", "g5.2xlarge", "p3.2xlarge"}},
		{"With spaces", "g5.xlarge, g5.2xlarge, p3.2xlarge", []string{"g5.xlarge", "g5.2xlarge", "p3.2xlarge"}},
		{"Empty string", "", []string{}},
		{"Only spaces", "   ", []string{}},
		{"Trailing comma", "g5.xlarge,", []string{"g5.xlarge"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseInstanceTypes(tt.input)
			assert.NoError(t, err)

			// Handle nil vs empty slice
			if len(tt.expected) == 0 {
				assert.Empty(t, result)
			} else {
				// Compare just the Type field of each InstanceSpec
				var resultTypes []string
				for _, spec := range result {
					resultTypes = append(resultTypes, spec.Type)
				}
				assert.Equal(t, tt.expected, resultTypes)
			}
		})
	}
}

func TestGPUCreateOptions(t *testing.T) {
	opts := GPUCreateOptions{
		Name: "my-instance",
		InstanceTypes: []InstanceSpec{
			{Type: "g5.xlarge", DiskGB: 500},
			{Type: "g5.2xlarge"},
		},
		Count:    2,
		Parallel: 3,
		Detached: true,
	}

	assert.Equal(t, "my-instance", opts.Name)
	assert.Len(t, opts.InstanceTypes, 2)
	assert.Equal(t, "g5.xlarge", opts.InstanceTypes[0].Type)
	assert.Equal(t, 500.0, opts.InstanceTypes[0].DiskGB)
	assert.Equal(t, "g5.2xlarge", opts.InstanceTypes[1].Type)
	assert.Equal(t, 0.0, opts.InstanceTypes[1].DiskGB)
	assert.Equal(t, 2, opts.Count)
	assert.Equal(t, 3, opts.Parallel)
	assert.True(t, opts.Detached)
}

func TestResolveWorkspaceUserOptionsStandard(t *testing.T) {
	user := &entity.User{
		ID:             "user-123",
		GlobalUserType: "Standard",
	}

	options := &store.CreateWorkspacesOptions{}
	result := resolveWorkspaceUserOptions(options, user)

	assert.Equal(t, store.UserWorkspaceTemplateID, result.WorkspaceTemplateID)
	assert.Equal(t, store.UserWorkspaceClassID, result.WorkspaceClassID)
}

func TestResolveWorkspaceUserOptionsAdmin(t *testing.T) {
	user := &entity.User{
		ID:             "user-123",
		GlobalUserType: "Admin",
	}

	options := &store.CreateWorkspacesOptions{}
	result := resolveWorkspaceUserOptions(options, user)

	assert.Equal(t, store.DevWorkspaceTemplateID, result.WorkspaceTemplateID)
	assert.Equal(t, store.DevWorkspaceClassID, result.WorkspaceClassID)
}

func TestResolveWorkspaceUserOptionsPreserveExisting(t *testing.T) {
	user := &entity.User{
		ID:             "user-123",
		GlobalUserType: "Standard",
	}

	options := &store.CreateWorkspacesOptions{
		WorkspaceTemplateID: "custom-template",
		WorkspaceClassID:    "custom-class",
	}
	result := resolveWorkspaceUserOptions(options, user)

	// Should preserve existing values
	assert.Equal(t, "custom-template", result.WorkspaceTemplateID)
	assert.Equal(t, "custom-class", result.WorkspaceClassID)
}

func TestMockGPUCreateStoreBasics(t *testing.T) {
	mock := NewMockGPUCreateStore()

	user, err := mock.GetCurrentUser()
	assert.NoError(t, err)
	assert.Equal(t, "user-123", user.ID)

	org, err := mock.GetActiveOrganizationOrDefault()
	assert.NoError(t, err)
	assert.Equal(t, "org-123", org.ID)
}

func TestMockGPUCreateStoreCreateWorkspace(t *testing.T) {
	mock := NewMockGPUCreateStore()

	options := store.NewCreateWorkspacesOptions("cluster-1", "test-instance")
	options.WithInstanceType("g5.xlarge")

	ws, err := mock.CreateWorkspace("org-123", options)
	assert.NoError(t, err)
	assert.Equal(t, "test-instance", ws.Name)
	assert.Equal(t, "g5.xlarge", ws.InstanceType)
	assert.Len(t, mock.CreatedWorkspaces, 1)
}

func TestMockGPUCreateStoreDeleteWorkspace(t *testing.T) {
	mock := NewMockGPUCreateStore()

	// First create a workspace
	options := store.NewCreateWorkspacesOptions("cluster-1", "test-instance")
	ws, _ := mock.CreateWorkspace("org-123", options)

	// Then delete it
	_, err := mock.DeleteWorkspace(ws.ID)
	assert.NoError(t, err)
	assert.Contains(t, mock.DeletedWorkspaceIDs, ws.ID)
}

func TestMockGPUCreateStoreTypeSpecificError(t *testing.T) {
	mock := NewMockGPUCreateStore()
	mock.CreateErrorTypes["g5.xlarge"] = assert.AnError

	options := store.NewCreateWorkspacesOptions("cluster-1", "test-instance")
	options.WithInstanceType("g5.xlarge")

	_, err := mock.CreateWorkspace("org-123", options)
	assert.Error(t, err)

	// Different type should work
	options2 := store.NewCreateWorkspacesOptions("cluster-1", "test-instance-2")
	options2.WithInstanceType("g5.2xlarge")

	ws, err := mock.CreateWorkspace("org-123", options2)
	assert.NoError(t, err)
	assert.NotNil(t, ws)
}

func TestCreateDryRunWithExplicitTypesDoesNotProvision(t *testing.T) {
	mock := NewMockGPUCreateStore()
	output := executeCreateAndCaptureOutput(t, mock, "dry-run-test", "--type", "g5.xlarge", "--dry-run")

	assert.Empty(t, mock.CreatedWorkspaces)
	assert.Contains(t, output, "g5.xlarge")
	assert.Contains(t, output, "location: us-east-1")
	assert.Contains(t, output, "sub-location: us-east-1a")
	assert.Equal(t, []bool{true}, mock.InstanceTypesIncludeCPU, "dry-run catalog must include CPU types")
}

func TestCreateDryRunAcceptsCanonicalRegion(t *testing.T) {
	mock := NewMockGPUCreateStore()
	output := executeCreateAndCaptureOutput(t, mock, "dry-run-region", "--type", "g5.xlarge", "--region", "US-EAST-1", "--dry-run")

	assert.Empty(t, mock.CreatedWorkspaces)
	assert.Contains(t, output, "location: us-east-1")
	assert.NotContains(t, output, "sub-location:", "an explicit location without a zone must clear the catalog default zone")
}

func TestCreateCPUDryRunShowsDefaultLocation(t *testing.T) {
	mock := NewMockGPUCreateStore()
	output := executeCreateAndCaptureOutput(t, mock, "cpu-dry-run", "--type", "n2d-standard-2", "--dry-run")

	assert.Empty(t, mock.CreatedWorkspaces)
	assert.Contains(t, output, "n2d-standard-2")
	assert.Contains(t, output, "location: asia-south1")
	assert.Contains(t, output, "sub-location: asia-south1-a")
}

func TestCreateCPUWithLocationAndSubLocation(t *testing.T) {
	mock := NewMockGPUCreateStore()
	mock.AllInstanceTypes = &gpusearch.AllInstanceTypesResponse{
		AllInstanceTypes: []gpusearch.InstanceType{
			{
				Type:               "n2d-standard-2",
				CloudCredID:        "cc-gcp-west",
				Location:           "asia-south1",
				SubLocation:        "asia-south1-a",
				AvailableLocations: []string{"asia-south1", "us-west2"},
			},
		},
	}

	output := executeCreateAndCaptureOutput(
		t,
		mock,
		"cpu-west",
		"--type", "n2d-standard-2",
		"--location", "us-west2",
		"--sub-location", "us-west2-a",
		"--detached",
	)

	require.Len(t, mock.CreatedOptions, 1)
	assert.Equal(t, "n2d-standard-2", mock.CreatedOptions[0].InstanceType)
	assert.Equal(t, "us-west2", mock.CreatedOptions[0].Location)
	assert.Equal(t, "us-west2-a", mock.CreatedOptions[0].SubLocation)
	assert.Equal(t, "cc-gcp-west", mock.CreatedOptions[0].CloudCredID)
	assert.Contains(t, output, "location: us-west2")
	assert.Contains(t, output, "sub-location: us-west2-a")
	payload, marshalErr := json.Marshal(mock.CreatedOptions[0])
	require.NoError(t, marshalErr)
	assert.Contains(t, string(payload), `"location":"us-west2"`)
	assert.Contains(t, string(payload), `"subLocation":"us-west2-a"`)
}

func TestCreateLocationFlagValidation(t *testing.T) {
	t.Run("conflicting aliases are rejected", func(t *testing.T) {
		mock := NewMockGPUCreateStore()
		cmd := NewCmdGPUCreate(terminal.New(), mock)
		cmd.SetArgs([]string{"conflict", "--type", "g5.xlarge", "--region", "us-east-1", "--location", "us-west-2", "--dry-run"})

		err := cmd.Execute()

		assert.ErrorContains(t, err, `--region "us-east-1" conflicts with --location "us-west-2"`)
		assert.Empty(t, mock.CreatedWorkspaces)
	})

	t.Run("sub-location requires a location", func(t *testing.T) {
		mock := NewMockGPUCreateStore()
		cmd := NewCmdGPUCreate(terminal.New(), mock)
		cmd.SetArgs([]string{"orphan-zone", "--type", "n2d-standard-2", "--sub-location", "us-west2-a", "--dry-run"})

		err := cmd.Execute()

		assert.ErrorContains(t, err, "--sub-location requires --location or --region")
		assert.Empty(t, mock.CreatedWorkspaces)
	})

	t.Run("equivalent aliases are accepted", func(t *testing.T) {
		mock := NewMockGPUCreateStore()
		output := executeCreateAndCaptureOutput(
			t,
			mock,
			"same-location",
			"--type", "g5.xlarge",
			"--region", "US-EAST-1",
			"--location", "us-east-1",
			"--dry-run",
		)

		assert.Contains(t, output, "location: us-east-1")
		assert.Empty(t, mock.CreatedWorkspaces)
	})
}

func TestCreateAutoSelectsTypeAndCredentialForRegion(t *testing.T) {
	mock := NewMockGPUCreateStore()
	mock.AllInstanceTypes = &gpusearch.AllInstanceTypesResponse{
		AllInstanceTypes: []gpusearch.InstanceType{
			{
				Type:               "g5.xlarge",
				CloudCredID:        "cc-west",
				AvailableLocations: []string{"us-west-2"},
			},
			{
				Type:               "g5.xlarge",
				CloudCredID:        "cc-east",
				AvailableLocations: []string{"us-east-1"},
			},
		},
	}
	cmd := NewCmdGPUCreate(terminal.New(), mock)
	cmd.SetArgs([]string{"auto-region", "--region", "US-EAST-1", "--detached"})

	err := cmd.Execute()

	require.NoError(t, err)
	require.Len(t, mock.CreatedOptions, 1)
	assert.Equal(t, "g5.xlarge", mock.CreatedOptions[0].InstanceType)
	assert.Equal(t, "us-east-1", mock.CreatedOptions[0].Location)
	assert.Equal(t, "cc-east", mock.CreatedOptions[0].CloudCredID)
}

func TestCreateRejectsUnknownRegion(t *testing.T) {
	mock := NewMockGPUCreateStore()
	cmd := NewCmdGPUCreate(terminal.New(), mock)
	cmd.SetArgs([]string{"bad-region", "--type", "g5.xlarge", "--region", "moon-1", "--dry-run"})

	err := cmd.Execute()

	assert.ErrorContains(t, err, `location "moon-1" is not available`)
	assert.Contains(t, err.Error(), "us-east-1")
	assert.Contains(t, err.Error(), "us-west-2")
	assert.Empty(t, mock.CreatedWorkspaces)
}

func TestCreateRejectsUnsupportedCPULocationWithSupportedList(t *testing.T) {
	mock := NewMockGPUCreateStore()
	cmd := NewCmdGPUCreate(terminal.New(), mock)
	cmd.SetArgs([]string{"cpu-wrong-location", "--type", "n2d-standard-2", "--location", "us-east-1", "--dry-run"})

	err := cmd.Execute()

	assert.ErrorContains(t, err, `location "us-east-1" is not available for instance type(s) n2d-standard-2`)
	assert.Contains(t, err.Error(), "asia-south1")
	assert.Contains(t, err.Error(), "us-west2")
	assert.NotContains(t, err.Error(), "us-west-2", "the error should list locations for the requested CPU type, not the entire catalog")
	assert.Empty(t, mock.CreatedWorkspaces)
}

func TestRegionValidation(t *testing.T) {
	items := []gpusearch.InstanceType{
		{Type: "g5.xlarge", AvailableLocations: []string{"us-west-2"}},
		{Type: "g5.xlarge", AvailableLocations: []string{"us-east-1"}},
		{Type: "g6.xlarge", AvailableLocations: []string{"eu-west-1"}},
	}

	region, err := canonicalRegion(" US-EAST-1 ", []InstanceSpec{{Type: "g5.xlarge"}}, items)
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", region)
	assert.NoError(t, validateTypesSupportRegion([]InstanceSpec{{Type: "g5.xlarge"}}, region, items))
	assert.ErrorContains(t, validateTypesSupportRegion([]InstanceSpec{{Type: "g6.xlarge"}}, region, items), "g6.xlarge")
	assert.NoError(t, validateTypesSupportRegion([]InstanceSpec{{Type: "g6.xlarge"}, {Type: "g5.xlarge"}}, region, items), "a compatible fallback keeps the chain usable")
	assert.NoError(t, validateTypesSupportRegion([]InstanceSpec{{Type: "private-type"}}, region, items), "types absent from the public catalog are validated by the authenticated create path")
}

func TestGetFilteredInstanceTypesDefaults(t *testing.T) {
	mock := NewMockGPUCreateStore()

	// Get instance types with no user filters (uses defaults):
	// - 24GB VRAM (>= 20GB total VRAM requirement)
	// - 500GB disk (>= 500GB requirement)
	// - A10G GPU = 8.6 capability (>= 8.0 requirement)
	// - 5m boot time (< 7m requirement)
	specs, err := getFilteredInstanceTypes(mock, &searchFilterFlags{})
	assert.NoError(t, err)
	assert.Len(t, specs, 1)
	assert.Equal(t, "g5.xlarge", specs[0].Type)
	assert.Equal(t, 500.0, specs[0].DiskGB) // Should use the instance's disk size
}

func TestGetFilteredInstanceTypesWithGPUName(t *testing.T) {
	mock := NewMockGPUCreateStore()

	// Filter by GPU name that matches the mock data
	specs, err := getFilteredInstanceTypes(mock, &searchFilterFlags{gpuName: "A10G"})
	assert.NoError(t, err)
	assert.Len(t, specs, 1)
	assert.Equal(t, "g5.xlarge", specs[0].Type)
}

func TestGetFilteredInstanceTypesNoMatch(t *testing.T) {
	mock := NewMockGPUCreateStore()

	// Filter by GPU name that doesn't match
	specs, err := getFilteredInstanceTypes(mock, &searchFilterFlags{gpuName: "H100"})
	assert.NoError(t, err)
	assert.Len(t, specs, 0)
}

func TestParseTableInput(t *testing.T) {
	tableInput := strings.Join([]string{
		"TYPE           TARGET_DISK  PROVIDER  DEFAULT_LOCATION          GPU    COUNT  VRAM/GPU  TOTAL VRAM  CAPABILITY  VCPUs  $/HR",
		"g5.xlarge      500          aws       us-east-1/us-east-1a      A10G   1      24 GB     24 GB       8.6         4      $1.01",
		"g5.2xlarge     500          aws       us-west-2                 A10G   1      24 GB     24 GB       8.6         8      $1.21",
		"p4d.24xlarge   1000         aws       us-east-1                 A100   8      40 GB     320 GB      8.0         96     $32.77",
		"",
		"Found 3 GPU instance types",
	}, "\n")

	specs := parseTableInput(tableInput)

	assert.Len(t, specs, 3)
	assert.Equal(t, "g5.xlarge", specs[0].Type)
	assert.Equal(t, 500.0, specs[0].DiskGB)
	assert.Equal(t, "g5.2xlarge", specs[1].Type)
	assert.Equal(t, 500.0, specs[1].DiskGB)
	assert.Equal(t, "p4d.24xlarge", specs[2].Type)
	assert.Equal(t, 1000.0, specs[2].DiskGB)
}

func TestParseTableInputCPU(t *testing.T) {
	// Simulated plain table output from `brev search cpu`
	tableInput := strings.Join([]string{
		" TYPE                   TARGET_DISK  PROVIDER  DEFAULT_LOCATION          VCPUS  RAM   ARCH    DISK       $/GB/MO  BOOT  FEATURES  $/HR",
		" n2d-standard-2         10           gcp       asia-south1/asia-south1-a  2      8     x86_64  10GB-16TB  $0.13    7m    SP        $0.05",
		" n1-standard-1          10           gcp       us-west2                   1      4     x86_64  10GB-16TB  $0.14    7m    SP        $0.06",
		" m8i-flex.8xlarge       500          aws       us-west-2                  32     128   x86_64  10GB-16TB  $0.10    7m    SRP       $1.93",
		"",
		"Found 3 CPU instance types",
	}, "\n")

	specs := parseTableInput(tableInput)

	assert.Len(t, specs, 3)
	assert.Equal(t, "n2d-standard-2", specs[0].Type)
	assert.Equal(t, 10.0, specs[0].DiskGB)
	assert.Equal(t, "n1-standard-1", specs[1].Type)
	assert.Equal(t, 10.0, specs[1].DiskGB)
	assert.Equal(t, "m8i-flex.8xlarge", specs[2].Type)
	assert.Equal(t, 500.0, specs[2].DiskGB)
}

func TestParseJSONInput(t *testing.T) {
	// Simulated JSON output from gpu-search --json
	jsonInput := `[
		{
			"type": "g5.xlarge",
			"provider": "aws",
			"gpu_name": "A10G",
			"location": "us-east-1",
			"sub_location": "us-east-1a",
			"target_disk_gb": 1000
		},
		{
			"type": "p4d.24xlarge",
			"provider": "aws",
			"gpu_name": "A100",
			"target_disk_gb": 500
		},
		{
			"type": "g6.xlarge",
			"provider": "aws",
			"gpu_name": "L4"
		}
	]`

	specs, err := parseJSONInput(jsonInput)
	assert.NoError(t, err)
	assert.Len(t, specs, 3)

	// Check first instance with disk
	assert.Equal(t, "g5.xlarge", specs[0].Type)
	assert.Equal(t, 1000.0, specs[0].DiskGB)
	assert.Empty(t, specs[0].Location, "search placement metadata must not silently become an explicit create override")
	assert.Empty(t, specs[0].SubLocation)

	// Check second instance with different disk
	assert.Equal(t, "p4d.24xlarge", specs[1].Type)
	assert.Equal(t, 500.0, specs[1].DiskGB)

	// Check third instance without disk (should be 0)
	assert.Equal(t, "g6.xlarge", specs[2].Type)
	assert.Equal(t, 0.0, specs[2].DiskGB)
}

func TestFormatInstanceSpecs(t *testing.T) {
	specs := []InstanceSpec{
		{Type: "g5.xlarge", DiskGB: 1000},
		{Type: "p4d.24xlarge", DiskGB: 0, Location: "us-west-2"},
		{Type: "g6.xlarge", DiskGB: 500, Location: "us-west2", SubLocation: "us-west2-a"},
	}

	result := formatInstanceSpecs(specs)
	assert.Equal(t, "g5.xlarge (1000GB disk), p4d.24xlarge (location: us-west-2), g6.xlarge (500GB disk, location: us-west2, sub-location: us-west2-a)", result)
}

func TestPollUntilReadyReportsWorkspaceFailureMessage(t *testing.T) {
	store := NewMockGPUCreateStore()
	store.Workspaces["ws-failed"] = &entity.Workspace{
		ID:            "ws-failed",
		Name:          "test",
		Status:        entity.Failure,
		StatusMessage: "unexpected end of JSON input",
	}

	ctx := &createContext{
		store: store,
		opts:  GPUCreateOptions{Timeout: time.Second},
	}

	err := ctx.pollUntilReady("ws-failed")

	assert.ErrorContains(t, err, "instance test failed: unexpected end of JSON input")
}

func TestInlineLaunchableLifeCycleScript(t *testing.T) {
	t.Run("fetches and inlines lifecycle script body", func(t *testing.T) {
		mockStore := NewMockGPUCreateStore()
		info := &store.LaunchableResponse{
			BuildRequest: store.LaunchableBuildRequest{
				VMBuild: &store.VMBuild{
					LifeCycleScriptAttr: &store.LifeCycleScriptAttr{ID: "ls-abc"},
				},
			},
		}

		err := inlineLaunchableLifeCycleScript(mockStore, "env-abc", info)

		assert.NoError(t, err)
		assert.Equal(t, []string{"ls-abc"}, mockStore.FetchedLifeCycleScriptIDs)
		assert.Equal(t, "echo mock-script", info.BuildRequest.VMBuild.LifeCycleScriptAttr.Script)
	})

	t.Run("skips fetch when info is nil", func(t *testing.T) {
		mockStore := NewMockGPUCreateStore()

		err := inlineLaunchableLifeCycleScript(mockStore, "env-abc", nil)

		assert.NoError(t, err)
		assert.Empty(t, mockStore.FetchedLifeCycleScriptIDs)
	})

	t.Run("container build skips lifecycle script fetch", func(t *testing.T) {
		mockStore := NewMockGPUCreateStore()
		info := &store.LaunchableResponse{
			BuildRequest: store.LaunchableBuildRequest{
				CustomContainer: &store.CustomContainer{ContainerURL: "nvcr.io/nvidia/test:latest"},
			},
		}

		err := inlineLaunchableLifeCycleScript(mockStore, "env-abc", info)

		assert.NoError(t, err)
		assert.Empty(t, mockStore.FetchedLifeCycleScriptIDs)
	})

	t.Run("skips fetch when launchable has no lifecycle script", func(t *testing.T) {
		mockStore := NewMockGPUCreateStore()
		info := &store.LaunchableResponse{
			BuildRequest: store.LaunchableBuildRequest{
				VMBuild: &store.VMBuild{ForceJupyterInstall: true},
			},
		}

		err := inlineLaunchableLifeCycleScript(mockStore, "env-abc", info)

		assert.NoError(t, err)
		assert.Empty(t, mockStore.FetchedLifeCycleScriptIDs)
		assert.Nil(t, info.BuildRequest.VMBuild.LifeCycleScriptAttr)
	})

	t.Run("skips fetch when script ID is empty", func(t *testing.T) {
		mockStore := NewMockGPUCreateStore()
		info := &store.LaunchableResponse{
			BuildRequest: store.LaunchableBuildRequest{
				VMBuild: &store.VMBuild{
					LifeCycleScriptAttr: &store.LifeCycleScriptAttr{Name: "stale"},
				},
			},
		}

		err := inlineLaunchableLifeCycleScript(mockStore, "env-abc", info)

		assert.NoError(t, err)
		assert.Empty(t, mockStore.FetchedLifeCycleScriptIDs)
		assert.Equal(t, "", info.BuildRequest.VMBuild.LifeCycleScriptAttr.Script)
	})
}

func TestValidateInstanceTypeAvailability(t *testing.T) {
	t.Run("returns nil when listing is unavailable", func(t *testing.T) {
		ctx := &createContext{}
		assert.NoError(t, ctx.validateInstanceTypeAvailability("hyperstack_H100x8_one"))
	})

	t.Run("returns nil when type has a cloud credential", func(t *testing.T) {
		ctx := &createContext{
			allInstanceTypes: &gpusearch.AllInstanceTypesResponse{
				AllInstanceTypes: []gpusearch.InstanceType{
					{
						Type:        "hyperstack_H100_sxm5x8",
						CloudCredID: "cc-1",
					},
				},
			},
		}
		assert.NoError(t, ctx.validateInstanceTypeAvailability("hyperstack_H100_sxm5x8"))
	})

	t.Run("returns invalid-type error for unknown type", func(t *testing.T) {
		ctx := &createContext{
			allInstanceTypes: &gpusearch.AllInstanceTypesResponse{
				AllInstanceTypes: []gpusearch.InstanceType{
					{Type: "hyperstack_H100_sxm5x8", CloudCredID: "cc-1"},
				},
			},
		}
		err := ctx.validateInstanceTypeAvailability("hyperstack_H100x8_one")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `"hyperstack_H100x8_one"`)
		assert.Contains(t, err.Error(), "not a recognized type")
		assert.Contains(t, err.Error(), "brev search")
	})

	t.Run("returns unavailable error for known type without a cloud credential", func(t *testing.T) {
		ctx := &createContext{
			allInstanceTypes: &gpusearch.AllInstanceTypesResponse{
				AllInstanceTypes: []gpusearch.InstanceType{
					{Type: "hyperstack_H100x8_NVLINK"},
				},
			},
		}
		err := ctx.validateInstanceTypeAvailability("hyperstack_H100x8_NVLINK")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `"hyperstack_H100x8_NVLINK"`)
		assert.Contains(t, err.Error(), "currently unavailable")
		assert.Contains(t, err.Error(), "brev search")
	})

	t.Run("returns a region-specific error when credentials exist only elsewhere", func(t *testing.T) {
		ctx := &createContext{
			opts: GPUCreateOptions{Region: "us-east-1"},
			allInstanceTypes: &gpusearch.AllInstanceTypesResponse{
				AllInstanceTypes: []gpusearch.InstanceType{
					{
						Type:               "hyperstack_H100_sxm5x8",
						CloudCredID:        "cc-west",
						AvailableLocations: []string{"us-west-2"},
					},
				},
			},
		}
		err := ctx.validateInstanceTypeAvailability("hyperstack_H100_sxm5x8")
		assert.ErrorContains(t, err, `unavailable in location "us-east-1"`)
		assert.Contains(t, err.Error(), "available locations: us-west-2")
	})

	t.Run("error type is ValidationError so no stack trace is appended", func(t *testing.T) {
		ctx := &createContext{
			allInstanceTypes: &gpusearch.AllInstanceTypesResponse{
				AllInstanceTypes: []gpusearch.InstanceType{},
			},
		}
		err := ctx.validateInstanceTypeAvailability("missing")
		assert.Error(t, err)
		var ve breverrors.ValidationError
		assert.ErrorAs(t, err, &ve)
		assert.NotContains(t, err.Error(), "gpucreate.go", "validation error should not include source-file traces")
	})
}

func TestCreateInstancesWithTypeSkipsInvalidType(t *testing.T) {
	mock := NewMockGPUCreateStore()
	ctx := &createContext{
		t:     terminal.New(),
		store: mock,
		opts:  GPUCreateOptions{Count: 1, Parallel: 1, Name: "jt-4"},
		org:   mock.Org,
		user:  mock.User,
		piped: true,
		allInstanceTypes: &gpusearch.AllInstanceTypesResponse{
			AllInstanceTypes: []gpusearch.InstanceType{
				{
					Type:        "hyperstack_H100_sxm5x8",
					CloudCredID: "cc-shadeform",
				},
			},
		},
	}
	ctx.logf = func(_ string, _ ...interface{}) {}

	result := ctx.createInstancesWithType(InstanceSpec{Type: "hyperstack_H100x8_one"}, 0, 1)

	assert.True(t, result.hadFailure, "expected hadFailure for an invalid instance type")
	assert.Empty(t, result.successes, "expected no successes for invalid type")
	assert.NoError(t, result.fatalError, "invalid type should not be fatal — caller may try the next type")
	assert.Empty(t, mock.CreatedWorkspaces, "CreateWorkspace must not be called when the type is unrecognized")
}

func TestCreateInstancesWithTypeSkipsUnavailableType(t *testing.T) {
	mock := NewMockGPUCreateStore()
	ctx := &createContext{
		t:     terminal.New(),
		store: mock,
		opts:  GPUCreateOptions{Count: 1, Parallel: 1, Name: "jt-4"},
		org:   mock.Org,
		user:  mock.User,
		piped: true,
		allInstanceTypes: &gpusearch.AllInstanceTypesResponse{
			AllInstanceTypes: []gpusearch.InstanceType{
				{Type: "hyperstack_H100x8_NVLINK"},
			},
		},
	}
	ctx.logf = func(_ string, _ ...interface{}) {}

	result := ctx.createInstancesWithType(InstanceSpec{Type: "hyperstack_H100x8_NVLINK"}, 0, 1)

	assert.True(t, result.hadFailure, "expected hadFailure for an unavailable instance type")
	assert.Empty(t, result.successes)
	assert.Empty(t, mock.CreatedWorkspaces, "CreateWorkspace must not be called when no cloud credential is available")
}

func TestCreateInstancesWithTypeSetsCloudCredIDFromCatalog(t *testing.T) {
	mock := NewMockGPUCreateStore()
	ctx := &createContext{
		t:     terminal.New(),
		store: mock,
		opts:  GPUCreateOptions{Count: 1, Parallel: 1, Name: "jt-4", Region: "us-east-1"},
		org:   mock.Org,
		user:  mock.User,
		piped: true,
		allInstanceTypes: &gpusearch.AllInstanceTypesResponse{
			AllInstanceTypes: []gpusearch.InstanceType{
				{
					Type:               "hyperstack_H100_sxm5x8",
					CloudCredID:        "cc-shadeform-west",
					AvailableLocations: []string{"us-west-2"},
				},
				{
					Type:               "hyperstack_H100_sxm5x8",
					CloudCredID:        "cc-shadeform-east",
					AvailableLocations: []string{"us-east-1"},
				},
			},
		},
	}
	ctx.logf = func(_ string, _ ...interface{}) {}

	result := ctx.createInstancesWithType(InstanceSpec{Type: "hyperstack_H100_sxm5x8"}, 0, 1)

	assert.False(t, result.hadFailure)
	require.Len(t, mock.CreatedOptions, 1)
	assert.Equal(t, "cc-shadeform-east", mock.CreatedOptions[0].CloudCredID)
	assert.Equal(t, "us-east-1", mock.CreatedOptions[0].Location)
}

func TestCreateInstancesWithTypeBypassesValidationForLaunchable(t *testing.T) {
	mock := NewMockGPUCreateStore()
	ctx := &createContext{
		t:     terminal.New(),
		store: mock,
		opts: GPUCreateOptions{
			Count:        1,
			Parallel:     1,
			Name:         "jt-4",
			LaunchableID: "env-abc",
			Region:       "us-east-1",
			LaunchableInfo: &store.LaunchableResponse{
				ID:   "env-abc",
				Name: "test-launchable",
				CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
					CloudCredID:  "cc-from-launchable",
					InstanceType: "n2-standard-4",
					Location:     "eu-west-1",
					SubLocation:  "eu-west-1a",
				},
			},
		},
		org:   mock.Org,
		user:  mock.User,
		piped: true,
		allInstanceTypes: &gpusearch.AllInstanceTypesResponse{
			AllInstanceTypes: []gpusearch.InstanceType{}, // launchable's type is not in the org listing
		},
	}
	ctx.logf = func(_ string, _ ...interface{}) {}

	result := ctx.createInstancesWithType(InstanceSpec{Type: "n2-standard-4"}, 0, 1)

	assert.False(t, result.hadFailure, "launchable should not be blocked by pre-flight validation")
	assert.Len(t, result.successes, 1, "expected the launchable instance to be created")
	assert.Len(t, mock.CreatedWorkspaces, 1)
	assert.Equal(t, "cc-from-launchable", mock.CreatedOptions[0].CloudCredID, "private launchable types absent from the catalog keep their configured credential")
	assert.Equal(t, "us-east-1", mock.CreatedOptions[0].Location, "explicit region should override the launchable default")
	assert.Empty(t, mock.CreatedOptions[0].SubLocation, "a sub-location from the launchable's original region must not leak into the override")
}

func TestCreateLaunchableRejectsCredentialFromDifferentRegion(t *testing.T) {
	mock := NewMockGPUCreateStore()
	ctx := &createContext{
		t:     terminal.New(),
		store: mock,
		opts: GPUCreateOptions{
			Count:        1,
			Parallel:     1,
			Name:         "jt-4",
			LaunchableID: "env-abc",
			Region:       "us-east-1",
			LaunchableInfo: &store.LaunchableResponse{
				ID: "env-abc",
				CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
					CloudCredID:  "cc-west",
					InstanceType: "g5.xlarge",
					Location:     "us-west-2",
				},
			},
		},
		org:   mock.Org,
		user:  mock.User,
		piped: true,
		allInstanceTypes: &gpusearch.AllInstanceTypesResponse{
			AllInstanceTypes: []gpusearch.InstanceType{
				{
					Type:               "g5.xlarge",
					CloudCredID:        "cc-west",
					AvailableLocations: []string{"us-west-2"},
				},
			},
		},
	}
	ctx.logf = func(_ string, _ ...interface{}) {}

	result := ctx.createInstancesWithType(InstanceSpec{Type: "g5.xlarge"}, 0, 1)

	assert.True(t, result.hadFailure)
	assert.Empty(t, mock.CreatedOptions)
}
