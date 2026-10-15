package gpucreate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// backgroundContext and canceledContext are context builders for table-driven
// tests that vary the parent context.
func backgroundContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func canceledContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx, cancel
}

// expiredDeadlineContext returns a parent whose deadline has already passed.
func expiredDeadlineContext() (context.Context, context.CancelFunc) {
	return context.WithDeadline(context.Background(), time.Now().Add(-time.Minute))
}

// MockGPUCreateStore is a mock implementation of GPUCreateStore for testing
type MockGPUCreateStore struct {
	User                      *entity.User
	Org                       *entity.Organization
	CreateErrorTypes          map[string]error // Errors for specific instance types
	EnvironmentStatuses       map[string]devplanev1.EnvironmentStatus
	EnvironmentStatusMessages map[string]string
	CreatedOptions            []*store.CreateWorkspacesOptions
	CreatedWorkspaces         []*entity.Workspace
	DeletedWorkspaceIDs       []string
	FetchedLifeCycleScriptIDs []string
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
		CreateErrorTypes:    make(map[string]error),
		EnvironmentStatuses: make(map[string]devplanev1.EnvironmentStatus),
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

func (m *MockGPUCreateStore) GetEnvironment(_ context.Context, environmentID string) (*devplanev1.Environment, error) {
	env := &devplanev1.Environment{EnvironmentId: environmentID}
	if status, ok := m.EnvironmentStatuses[environmentID]; ok {
		env.Status = status
	}
	if msg, ok := m.EnvironmentStatusMessages[environmentID]; ok {
		env.Instance = &devplanev1.Instance{
			ProvisionStatus: &devplanev1.CreateAttemptStatus{Message: msg},
		}
	}
	return env, nil
}

func (m *MockGPUCreateStore) CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error) {
	// Check for type-specific errors first
	if err, ok := m.CreateErrorTypes[options.InstanceType]; ok {
		return nil, err
	}

	ws := &entity.Workspace{
		ID:           "ws-" + options.Name,
		Name:         options.Name,
		InstanceType: options.InstanceType,
		Status:       entity.Running,
	}
	m.CreatedOptions = append(m.CreatedOptions, options)
	m.CreatedWorkspaces = append(m.CreatedWorkspaces, ws)
	return ws, nil
}

func (m *MockGPUCreateStore) DeleteWorkspace(workspaceID string) (*entity.Workspace, error) {
	m.DeletedWorkspaceIDs = append(m.DeletedWorkspaceIDs, workspaceID)
	return nil, nil
}

func (m *MockGPUCreateStore) GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error) {
	return []entity.Workspace{}, nil
}

func (m *MockGPUCreateStore) GetAllInstanceTypesWithCloudCreds(orgID string) (*gpusearch.AllInstanceTypesResponse, error) {
	return &gpusearch.AllInstanceTypesResponse{
		AllInstanceTypes: []gpusearch.InstanceType{
			{Type: "test.fail.quota", CloudCredID: "testkube-brev-test"},
			{Type: "g5.xlarge", CloudCredID: "cc-shadeform"},
		},
	}, nil
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

func (m *MockGPUCreateStore) GetInstanceTypes(_ bool) (*gpusearch.InstanceTypesResponse, error) {
	// Return a default set of instance types for testing
	return &gpusearch.InstanceTypesResponse{
		Items: []gpusearch.InstanceType{
			{
				Type: "g5.xlarge",
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
		},
	}, nil
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
			result, err := parseInstanceTypes(tt.input, false, strings.NewReader(""))
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

func TestParseInstanceTypesStdin(t *testing.T) {
	// --type is authoritative: stdin (even with data) is ignored and never read.
	specs, err := parseInstanceTypes("g5.xlarge", true, strings.NewReader("should-be-ignored\n"))
	assert.NoError(t, err)
	assert.Equal(t, []InstanceSpec{{Type: "g5.xlarge"}}, specs)

	// No --type and no --stdin: stdin is not read; caller falls back to the default GPU search.
	specs, err = parseInstanceTypes("", false, strings.NewReader("g5.xlarge\n"))
	assert.NoError(t, err)
	assert.Empty(t, specs)

	// --stdin with piped table data: types are parsed from stdin.
	specs, err = parseInstanceTypes("", true, strings.NewReader("g5.xlarge\ng4dn.xlarge\n"))
	assert.NoError(t, err)
	var got []string
	for _, s := range specs {
		got = append(got, s.Type)
	}
	assert.Equal(t, []string{"g5.xlarge", "g4dn.xlarge"}, got)

	// --stdin with piped JSON data: types are parsed from stdin.
	specs, err = parseInstanceTypes("", true, strings.NewReader(`[{"type":"p4d.24xlarge"}]`))
	assert.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, "p4d.24xlarge", specs[0].Type)

	// --stdin set but nothing arrives on stdin: fail loudly.
	_, err = parseInstanceTypes("", true, strings.NewReader("   \n"))
	assert.Error(t, err)

	// --stdin input that parses to zero instance types (e.g. JSON []): fail loudly.
	_, err = parseInstanceTypes("", true, strings.NewReader("[]"))
	assert.Error(t, err)
}

func TestResolveWorkspaceUserOptions(t *testing.T) {
	cases := []struct {
		name           string
		globalUserType string
		existing       *store.CreateWorkspacesOptions
		wantTemplate   string
		wantClass      string
	}{
		{
			name:           "standard user gets user template",
			globalUserType: "Standard",
			wantTemplate:   store.UserWorkspaceTemplateID,
			wantClass:      store.UserWorkspaceClassID,
		},
		{
			name:           "admin user gets dev template",
			globalUserType: "Admin",
			wantTemplate:   store.DevWorkspaceTemplateID,
			wantClass:      store.DevWorkspaceClassID,
		},
		{
			name:           "existing values are preserved",
			globalUserType: "Standard",
			existing: &store.CreateWorkspacesOptions{
				WorkspaceTemplateID: "custom-template",
				WorkspaceClassID:    "custom-class",
			},
			wantTemplate: "custom-template",
			wantClass:    "custom-class",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := tc.existing
			if options == nil {
				options = &store.CreateWorkspacesOptions{}
			}

			result := resolveWorkspaceUserOptions(options, &entity.User{
				ID:             "user-123",
				GlobalUserType: entity.GlobalUserType(tc.globalUserType),
			})

			assert.Equal(t, tc.wantTemplate, result.WorkspaceTemplateID)
			assert.Equal(t, tc.wantClass, result.WorkspaceClassID)
		})
	}
}

// TestCreateDryRunWithExplicitTypesDoesNotProvision covers --dry-run end to
// end: flags parse, no CreateWorkspace call is made.
func TestCreateDryRunWithExplicitTypesDoesNotProvision(t *testing.T) {
	mock := NewMockGPUCreateStore()
	term := terminal.New()

	cmd := NewCmdGPUCreate(term, mock)
	cmd.SetArgs([]string{"dry-run-test", "--type", "g5.xlarge", "--dry-run"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Empty(t, mock.CreatedWorkspaces)
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

// TestParseTableInput covers both `brev search` table shapes (GPU and CPU).
func TestParseTableInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []InstanceSpec
	}{
		{
			name: "gpu table",
			input: strings.Join([]string{
				"TYPE           TARGET_DISK  GPU    COUNT  VRAM/GPU  TOTAL VRAM  CAPABILITY  VCPUs  $/HR",
				"g5.xlarge      500          A10G   1      24 GB     24 GB       8.6         4      $1.01",
				"g5.2xlarge     500          A10G   1      24 GB     24 GB       8.6         8      $1.21",
				"p4d.24xlarge   1000         A100   8      40 GB     320 GB      8.0         96     $32.77",
				"",
				"Found 3 GPU instance types",
			}, "\n"),
			want: []InstanceSpec{
				{Type: "g5.xlarge", DiskGB: 500},
				{Type: "g5.2xlarge", DiskGB: 500},
				{Type: "p4d.24xlarge", DiskGB: 1000},
			},
		},
		{
			name: "cpu table",
			input: strings.Join([]string{
				" TYPE                   TARGET_DISK  PROVIDER  VCPUS  RAM   ARCH    DISK       $/GB/MO  BOOT  FEATURES  $/HR",
				" n2d-highcpu-2          10           gcp           2  2     x86_64  10GB-16TB  $0.13    7m    SP        $0.05",
				" n1-standard-1          10           gcp           1  4     x86_64  10GB-16TB  $0.14    7m    SP        $0.06",
				" m8i-flex.8xlarge       500          aws          32  128   x86_64  10GB-16TB  $0.10    7m    SRP       $1.93",
				"",
				"Found 3 CPU instance types",
			}, "\n"),
			want: []InstanceSpec{
				{Type: "n2d-highcpu-2", DiskGB: 10},
				{Type: "n1-standard-1", DiskGB: 10},
				{Type: "m8i-flex.8xlarge", DiskGB: 500},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseTableInput(tc.input))
		})
	}
}

func TestParseJSONInput(t *testing.T) {
	// Simulated JSON output from gpu-search --json
	jsonInput := `[
		{
			"type": "g5.xlarge",
			"provider": "aws",
			"gpu_name": "A10G",
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
		{Type: "p4d.24xlarge", DiskGB: 0},
		{Type: "g6.xlarge", DiskGB: 500},
	}

	result := formatInstanceSpecs(specs)
	assert.Equal(t, "g5.xlarge (1000GB disk), p4d.24xlarge, g6.xlarge (500GB disk)", result)
}

// TestPollUntilReady covers the readiness-wait behavior across ctx states and
// environment status outcomes.
func TestPollUntilReady(t *testing.T) {
	cases := []struct {
		name        string
		status      devplanev1.EnvironmentStatus
		instanceMsg string
		parentCtx   func() (context.Context, context.CancelFunc)
		wantErrIs   error
		wantErrHas  string
	}{
		{
			name:      "cancellation propagates immediately",
			status:    devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING,
			parentCtx: canceledContext,
			wantErrIs: context.Canceled,
		},
		{
			name:      "expired parent deadline is not a readiness timeout",
			status:    devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING,
			parentCtx: expiredDeadlineContext,
			wantErrIs: context.DeadlineExceeded,
		},
		{
			name:        "terminal failure returns message",
			status:      devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED,
			instanceMsg: "unexpected end of JSON input",
			parentCtx:   backgroundContext,
			wantErrHas:  "instance ws-1 failed: unexpected end of JSON input",
		},
		{
			name:       "failure without provider message falls back to label",
			status:     devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED,
			parentCtx:  backgroundContext,
			wantErrHas: "instance ws-1 failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewMockGPUCreateStore()
			store.EnvironmentStatuses["ws-1"] = tc.status
			if tc.instanceMsg != "" {
				store.EnvironmentStatusMessages = map[string]string{"ws-1": tc.instanceMsg}
			}

			ctx := &createContext{
				t:     terminal.New(),
				store: store,
				opts:  GPUCreateOptions{Timeout: time.Minute},
				logf:  func(string, ...interface{}) {},
			}

			parentCtx, cancel := tc.parentCtx()
			defer cancel()

			err := ctx.pollUntilReady(parentCtx, "ws-1")

			require.Error(t, err)
			if tc.wantErrIs != nil {
				assert.ErrorIs(t, err, tc.wantErrIs)
			} else {
				assert.ErrorContains(t, err, tc.wantErrHas)
			}
		})
	}
}

// TestWaitForInstances covers how the wait aggregates per-instance outcomes.
func TestWaitForInstances(t *testing.T) {
	quietLogf := func(string, ...interface{}) {}

	cases := []struct {
		name         string
		workspaces   []*entity.Workspace
		statuses     map[string]devplanev1.EnvironmentStatus
		instanceMsgs map[string]string
		timeout      time.Duration
		parentCtx    func() (context.Context, context.CancelFunc)
		wantErr      error
		wantErrHas   []string
		wantNoError  bool
	}{
		{
			name:        "timeout on every instance is a warning, not an error",
			workspaces:  []*entity.Workspace{{ID: "ws-1", Name: "one"}},
			timeout:     0, // deadline already elapsed: immediate timeout
			parentCtx:   backgroundContext,
			wantNoError: true,
		},
		{
			name:       "terminal failure is reported with instance and message",
			workspaces: []*entity.Workspace{{ID: "ws-failed", Name: "failed-instance"}},
			statuses: map[string]devplanev1.EnvironmentStatus{
				"ws-failed": devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED,
			},
			instanceMsgs: map[string]string{
				"ws-failed": "out of quota in the region fulfill the request, InsufficientQuota",
			},
			timeout:    time.Second,
			parentCtx:  backgroundContext,
			wantErrHas: []string{"1/1 instance(s) failed to become ready", "ws-failed", "InsufficientQuota"},
		},
		{
			name:       "cancellation aborts remaining instances",
			workspaces: []*entity.Workspace{{ID: "ws-1", Name: "first"}, {ID: "ws-2", Name: "second"}},
			timeout:    time.Minute,
			parentCtx:  canceledContext,
			wantErr:    context.Canceled,
		},
		{
			name:       "expired parent deadline is not swallowed as warning",
			workspaces: []*entity.Workspace{{ID: "ws-1", Name: "one"}},
			timeout:    time.Minute,
			parentCtx:  expiredDeadlineContext,
			wantErr:    context.DeadlineExceeded,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewMockGPUCreateStore()
			store.EnvironmentStatuses = tc.statuses
			store.EnvironmentStatusMessages = tc.instanceMsgs

			ctx := &createContext{
				t:     terminal.New(),
				store: store,
				opts:  GPUCreateOptions{Timeout: tc.timeout},
				logf:  quietLogf,
			}

			parentCtx, cancel := tc.parentCtx()
			defer cancel()

			err := ctx.waitForInstances(parentCtx, tc.workspaces)

			if tc.wantNoError {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			}
			for _, want := range tc.wantErrHas {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}

// TestRunGPUCreateLifecycle covers the end-to-end create flow per environment
// status outcome, including how ctx cancellation surfaces.
func TestRunGPUCreateLifecycle(t *testing.T) {
	cases := []struct {
		name        string
		status      devplanev1.EnvironmentStatus
		instanceMsg string
		parentCtx   func() (context.Context, context.CancelFunc)
		wantErrIs   error
		wantErrHas  []string
		wantNoError bool
	}{
		{
			name:        "ready environment succeeds",
			status:      devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING,
			parentCtx:   backgroundContext,
			wantNoError: true,
		},
		{
			name:        "failed environment reports failure",
			status:      devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED,
			instanceMsg: "out of quota in the region fulfill the request, InsufficientQuota",
			parentCtx:   backgroundContext,
			wantErrHas:  []string{"failed to become ready", "InsufficientQuota"},
		},
		{
			name:      "canceled context aborts before the success summary",
			status:    devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING,
			parentCtx: canceledContext,
			wantErrIs: context.Canceled,
		},
		{
			name:      "expired parent deadline aborts before the success summary",
			status:    devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING,
			parentCtx: expiredDeadlineContext,
			wantErrIs: context.DeadlineExceeded,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockGPUCreateStore()
			mock.EnvironmentStatuses["ws-repro"] = tc.status
			if tc.instanceMsg != "" {
				mock.EnvironmentStatusMessages = map[string]string{"ws-repro": tc.instanceMsg}
			}

			parentCtx, cancel := tc.parentCtx()
			defer cancel()

			err := RunGPUCreate(parentCtx, terminal.New(), mock, GPUCreateOptions{
				Name:          "repro",
				Count:         1,
				Parallel:      1,
				InstanceTypes: []InstanceSpec{{Type: "g5.xlarge"}},
				Timeout:       time.Minute,
			})

			if tc.wantNoError {
				require.NoError(t, err)
				require.Len(t, mock.CreatedWorkspaces, 1)
				return
			}
			require.Error(t, err)
			if tc.wantErrIs != nil {
				assert.ErrorIs(t, err, tc.wantErrIs)
			}
			for _, want := range tc.wantErrHas {
				assert.ErrorContains(t, err, want)
			}
		})
	}
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

// TestCreateInstancesWithType covers per-type pre-flight validation outcomes
// and how the cloud credential is chosen.
func TestCreateInstancesWithType(t *testing.T) {
	catalog := func(types ...gpusearch.InstanceType) *gpusearch.AllInstanceTypesResponse {
		return &gpusearch.AllInstanceTypesResponse{AllInstanceTypes: types}
	}

	cases := []struct {
		name          string
		spec          InstanceSpec
		opts          GPUCreateOptions
		catalogTypes  *gpusearch.AllInstanceTypesResponse
		wantHadFail   bool
		wantSuccesses int
		wantCreated   int
		wantCredID    string
	}{
		{
			name:         "unrecognized type fails without creating",
			spec:         InstanceSpec{Type: "hyperstack_H100x8_one"},
			catalogTypes: catalog(gpusearch.InstanceType{Type: "hyperstack_H100_sxm5x8", CloudCredID: "cc-shadeform"}),
			wantHadFail:  true,
		},
		{
			name:         "type without cloud credential fails without creating",
			spec:         InstanceSpec{Type: "hyperstack_H100x8_NVLINK"},
			catalogTypes: catalog(gpusearch.InstanceType{Type: "hyperstack_H100x8_NVLINK"}),
			wantHadFail:  true,
		},
		{
			name:          "catalog credential is applied to the create request",
			spec:          InstanceSpec{Type: "hyperstack_H100_sxm5x8"},
			catalogTypes:  catalog(gpusearch.InstanceType{Type: "hyperstack_H100_sxm5x8", CloudCredID: "cc-shadeform"}),
			wantSuccesses: 1,
			wantCreated:   1,
			wantCredID:    "cc-shadeform",
		},
		{
			name: "launchable bypasses validation and uses launchable credential",
			spec: InstanceSpec{Type: "n2-standard-4"},
			opts: GPUCreateOptions{
				LaunchableID: "env-abc",
				LaunchableInfo: &store.LaunchableResponse{
					ID:   "env-abc",
					Name: "test-launchable",
					CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
						CloudCredID:  "cc-from-launchable",
						InstanceType: "n2-standard-4",
					},
				},
			},
			catalogTypes:  catalog(), // launchable's type is not in the org listing
			wantSuccesses: 1,
			wantCreated:   1,
			wantCredID:    "cc-from-launchable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockGPUCreateStore()
			opts := tc.opts
			opts.Count, opts.Parallel, opts.Name = 1, 1, "jt-4"

			ctx := &createContext{
				t:                terminal.New(),
				store:            mock,
				opts:             opts,
				org:              mock.Org,
				user:             mock.User,
				piped:            true,
				allInstanceTypes: tc.catalogTypes,
				logf:             func(string, ...interface{}) {},
			}

			result := ctx.createInstancesWithType(tc.spec, 0, 1)

			assert.Equal(t, tc.wantHadFail, result.hadFailure)
			assert.Len(t, result.successes, tc.wantSuccesses)
			assert.Len(t, mock.CreatedWorkspaces, tc.wantCreated)
			if tc.wantCredID != "" {
				require.Len(t, mock.CreatedOptions, 1)
				assert.Equal(t, tc.wantCredID, mock.CreatedOptions[0].CloudCredID)
			}
			if tc.wantHadFail {
				assert.NoError(t, result.fatalError, "per-type failure should not be fatal — caller may try the next type")
			}
		})
	}
}
