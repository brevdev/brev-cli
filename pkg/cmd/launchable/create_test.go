package launchable

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCreateOptions(t *testing.T) {
	t.Run("compose requires exactly one source", func(t *testing.T) {
		err := validateCreateOptions(launchableCreateOptions{
			Name:         "demo",
			ViewAccess:   "public",
			Mode:         "compose",
			InstanceType: "g5.xlarge",
			ComposeURL:   "https://github.com/acme/demo/blob/main/docker-compose.yml",
			ComposeYAML:  "services: {}",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "exactly one")
	})

	t.Run("vm rejects compose sources", func(t *testing.T) {
		err := validateCreateOptions(launchableCreateOptions{
			Name:            "demo",
			ViewAccess:      "public",
			Mode:            "vm",
			InstanceType:    "g5.xlarge",
			ComposeFilePath: "./docker-compose.yml",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "only valid with --mode compose")
	})
}

func TestValidateLifecycleScript(t *testing.T) {
	err := validateLifecycleScript("echo hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "#!/bin/bash")

	err = validateLifecycleScript("#!/bin/bash\necho hello")
	require.NoError(t, err)
}

func TestParseSecureLinks(t *testing.T) {
	links, err := parseSecureLinks([]string{
		"name=web,port=8080,cta=true,cta-label=Launch",
		"name=admin,port=3000",
	})
	require.NoError(t, err)
	require.Len(t, links, 2)
	assert.Equal(t, "web", links[0].Name)
	assert.True(t, links[0].CTA)
	assert.Equal(t, "Launch", links[0].CTALabel)

	_, err = parseSecureLinks([]string{
		"name=bad_name,port=8080",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid secure link name")
}

func TestValidatePortConflicts(t *testing.T) {
	err := validatePortConflicts(
		[]secureLinkSpec{{Name: "web", Port: 8080}},
		[]firewallRuleSpec{{Ports: "8000-8100", StartPort: 8000, EndPort: 8100, AllowedIPs: "all"}},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicts")
}

func TestResolveStorageValue(t *testing.T) {
	instance := gpusearch.InstanceType{
		Type: "g5.xlarge",
		SupportedStorage: []gpusearch.Storage{
			{MinSize: "100GiB", MaxSize: "16000GiB"},
		},
	}

	value, err := resolveStorageValue(instance, "")
	require.NoError(t, err)
	assert.Equal(t, "256", value)

	value, err = resolveStorageValue(instance, "512")
	require.NoError(t, err)
	assert.Equal(t, "512", value)
}

func TestBuildCreateEnvironmentLaunchableRequest(t *testing.T) {
	req := buildCreateEnvironmentLaunchableRequest(
		launchableCreateOptions{
			Name:        "demo",
			Description: "desc",
			ViewAccess:  "organization",
			Mode:        "compose",
			Jupyter:     false,
		},
		gpusearch.InstanceType{
			Type: "g5.xlarge",
			WorkspaceGroups: []gpusearch.WorkspaceGroup{
				{ID: "wg-1"},
			},
		},
		"256",
		"https://github.com/acme/demo/raw/main/README.md",
		"",
		&store.DockerCompose{
			FileURL:        "https://github.com/acme/demo/raw/main/docker-compose.yml",
			JupyterInstall: false,
		},
		[]secureLinkSpec{
			{Name: "web", Port: 3000, CTA: true, CTALabel: "Open Web"},
		},
		[]firewallRuleSpec{
			{Ports: "8000-8100", AllowedIPs: "user-ip", StartPort: 8000, EndPort: 8100},
		},
	)

	assert.Equal(t, "demo", req.Name)
	assert.Equal(t, "wg-1", req.CreateWorkspaceRequest.WorkspaceGroupID)
	assert.Equal(t, "256", req.CreateWorkspaceRequest.Storage)
	require.NotNil(t, req.BuildRequest.DockerCompose)
	assert.Equal(t, "https://github.com/acme/demo/raw/main/docker-compose.yml", req.BuildRequest.DockerCompose.FileURL)
	require.Len(t, req.BuildRequest.Ports, 1)
	assert.Equal(t, "true", req.BuildRequest.Ports[0].Labels["cta-enabled"])
	require.NotNil(t, req.File)
	assert.Equal(t, "./", req.File.Path)
}
