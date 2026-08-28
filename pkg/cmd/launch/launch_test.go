package launch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/cmd/gpusearch"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSecretResolver struct {
	latest map[string]string
	values map[string]string
	calls  []store.ManagedSecretReference
}

func (f *fakeSecretResolver) LatestVersion(_ context.Context, secretID string) (string, error) {
	version, ok := f.latest[secretID]
	if !ok {
		return "", errors.New("secret not found")
	}
	return version, nil
}

func (f *fakeSecretResolver) Value(_ context.Context, ref store.ManagedSecretReference) (string, error) {
	f.calls = append(f.calls, ref)
	value, ok := f.values[ref.SecretID+"@"+ref.VersionID]
	if !ok {
		return "", errors.New("secret value not found")
	}
	return value, nil
}

type fakeConfirmer struct {
	result bool
	calls  int
}

func (f *fakeConfirmer) ConfirmYesNo(_ string) bool {
	f.calls++
	return f.result
}

type recordedCommand struct {
	spec        commandSpec
	stdin       string
	composeYAML string
}

type fakeCommandRunner struct {
	paths    map[string]string
	commands []recordedCommand
	err      error
}

func (f *fakeCommandRunner) LookPath(file string) (string, error) {
	path, ok := f.paths[file]
	if !ok {
		return "", errors.New("not found")
	}
	return path, nil
}

func (f *fakeCommandRunner) Run(_ context.Context, spec commandSpec) error {
	recorded := recordedCommand{spec: spec}
	if spec.stdin != nil {
		contents, err := io.ReadAll(spec.stdin)
		if err != nil {
			return fmt.Errorf("read command stdin: %w", err)
		}
		recorded.stdin = string(contents)
	}
	if index := slices.Index(spec.args, "--file"); index >= 0 && index+1 < len(spec.args) {
		contents, err := os.ReadFile(spec.args[index+1])
		if err != nil {
			return fmt.Errorf("read temporary compose file: %w", err)
		}
		recorded.composeYAML = string(contents)
	}
	f.commands = append(f.commands, recorded)
	return f.err
}

type fakeComposeFetcher struct {
	contents []byte
	url      string
	err      error
}

type fakeLaunchStore struct {
	created        []*store.CreateWorkspacesOptions
	launchable     *store.LaunchableResponse
	lifecycle      *store.LifeCycleScriptResponse
	getCalls       []string
	lifecycleCalls [][2]string
}

func (f *fakeLaunchStore) GetAccessToken() (string, error) { return "token", nil }

func (f *fakeLaunchStore) GetCurrentUser() (*entity.User, error) {
	return &entity.User{ID: "user-1", GlobalUserType: "Standard"}, nil
}

func (f *fakeLaunchStore) GetAuthTokens() (*entity.AuthTokens, error) { return nil, nil }

func (f *fakeLaunchStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return &entity.Organization{ID: "org-1", Name: "Test Org"}, nil
}

func (f *fakeLaunchStore) GetWorkspace(workspaceID string) (*entity.Workspace, error) {
	return &entity.Workspace{ID: workspaceID, Status: entity.Running}, nil
}

func (f *fakeLaunchStore) CreateWorkspace(_ string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error) {
	f.created = append(f.created, options)
	return &entity.Workspace{
		ID:           "ws-1",
		Name:         options.Name,
		InstanceType: options.InstanceType,
		Status:       entity.Running,
	}, nil
}

func (f *fakeLaunchStore) DeleteWorkspace(string) (*entity.Workspace, error) {
	return &entity.Workspace{}, nil
}

func (f *fakeLaunchStore) GetWorkspaceByNameOrID(string, string) ([]entity.Workspace, error) {
	return nil, nil
}

func (f *fakeLaunchStore) GetAllInstanceTypesWithCloudCreds(string) (*gpusearch.AllInstanceTypesResponse, error) {
	return &gpusearch.AllInstanceTypesResponse{}, nil
}

func (f *fakeLaunchStore) GetLaunchable(id string) (*store.LaunchableResponse, error) {
	f.getCalls = append(f.getCalls, id)
	if f.launchable == nil {
		return nil, errors.New("not used")
	}
	return f.launchable, nil
}

func (f *fakeLaunchStore) GetLaunchableLifeCycleScript(launchableID string, scriptID string) (*store.LifeCycleScriptResponse, error) {
	f.lifecycleCalls = append(f.lifecycleCalls, [2]string{launchableID, scriptID})
	if f.lifecycle == nil {
		return nil, errors.New("not used")
	}
	return f.lifecycle, nil
}

func (f *fakeLaunchStore) RedeemCouponCode(string, string) (*store.RedeemCouponCodeResponse, error) {
	return &store.RedeemCouponCodeResponse{}, nil
}

func (f *fakeLaunchStore) GetInstanceTypes(bool) (*gpusearch.InstanceTypesResponse, error) {
	return &gpusearch.InstanceTypesResponse{}, nil
}

func (f *fakeComposeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	f.url = url
	return f.contents, f.err
}

func TestLaunchableExplainDisplaysBuildModeAndParameters(t *testing.T) {
	t.Setenv("BREV_CONSOLE_URL", "https://dev.brev.nvidia.com/cli-login")
	launchStore := &fakeLaunchStore{launchable: &store.LaunchableResponse{
		Name:        "vLLM Inference Server",
		Description: "Run an OpenAI-compatible inference server.",
		BuildRequest: store.LaunchableBuildRequest{
			DockerCompose: &store.DockerCompose{YamlString: "services: {}"},
			Parameters: []store.Parameter{
				{Name: "HF_TOKEN", Description: "Hugging Face access token.", Text: &store.TextParameter{}},
				{Name: "VLLM_ARGS", Description: "Additional arguments passed to vLLM.", Required: true, Text: &store.TextParameter{DefaultValue: "--max-model-len 16384 --gpu-memory-utilization 0.9"}},
				{Name: "MODEL", Required: true, Text: &store.TextParameter{DefaultValue: "Qwen/Qwen3-8B"}},
			},
		},
	}}
	var out bytes.Buffer
	cmd := newCmdLaunch(terminal.New(), launchStore, launchDeps{})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"env-3IEl5O5SlUAYJ9X1GKjAIZxoSnm", "--explain"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, `vLLM Inference Server

Run an OpenAI-compatible inference server.

URL: https://dev.brev.nvidia.com/launchable/deploy/now?launchableID=env-3IEl5O5SlUAYJ9X1GKjAIZxoSnm
Build mode: Docker Compose

Parameters:
  MODEL      (required, default: Qwen/Qwen3-8B)
  VLLM_ARGS  (required, default: --max-model-len 16384 --gpu-memory-utilization 0.9): Additional arguments passed to vLLM.
  HF_TOKEN   (optional): Hugging Face access token.
`, out.String())
	assert.Equal(t, []string{"env-3IEl5O5SlUAYJ9X1GKjAIZxoSnm"}, launchStore.getCalls)
	assert.Empty(t, launchStore.created)
}

func TestLaunchableExplainOmitsMissingDescription(t *testing.T) {
	t.Setenv("BREV_CONSOLE_URL", "https://brev.nvidia.com/cli-login")
	launchStore := &fakeLaunchStore{launchable: &store.LaunchableResponse{
		Name:         "Local setup",
		BuildRequest: store.LaunchableBuildRequest{VMBuild: &store.VMBuild{}},
	}}
	var out bytes.Buffer
	cmd := newCmdLaunch(terminal.New(), launchStore, launchDeps{})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"env-abc", "--explain"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, `Local setup

URL: https://brev.nvidia.com/launchable/deploy/now?launchableID=env-abc
Build mode: VM

Parameters: none
`, out.String())
}

func TestLaunchableDefinitionURLRejectsInvalidConsoleURL(t *testing.T) {
	_, err := launchableDefinitionURL("localhost:3000", "env-abc")
	assert.ErrorContains(t, err, "scheme and host")
}

func TestLaunchHelpWithIDRemainsCommandHelp(t *testing.T) {
	launchStore := &fakeLaunchStore{}
	var out bytes.Buffer
	cmd := newCmdLaunch(terminal.New(), launchStore, launchDeps{})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"env-abc", "-h"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Usage:")
	assert.Contains(t, out.String(), "launch <launchable-id-or-url>")
	assert.Empty(t, launchStore.getCalls)
}

func TestParseParameterInputs(t *testing.T) {
	values, err := parseParameterValues([]string{"MODEL=llama=3", "PORT=8080"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"MODEL": "llama=3", "PORT": "8080"}, values)

	secrets, err := parseParameterSecrets([]string{
		"TOKEN=secret-1",
		"PINNED=secret-2:version-3",
	})
	require.NoError(t, err)
	assert.Equal(t, store.ManagedSecretReference{SecretID: "secret-1"}, secrets["TOKEN"])
	assert.Equal(t, store.ManagedSecretReference{SecretID: "secret-2", VersionID: "version-3"}, secrets["PINNED"])

	_, err = parseParameterSecrets([]string{"TOKEN=secret-1:"})
	assert.ErrorContains(t, err, "invalid --param-secret")

	_, err = parseParameterSecrets([]string{"TOKEN=secret-1@version-3"})
	assert.ErrorContains(t, err, "use ':'")
}

func TestResolveParameterBindingsSupportsValuesDefaultsAndSecrets(t *testing.T) {
	resolver := &fakeSecretResolver{latest: map[string]string{"secret-1": "version-9"}}
	parameters := []store.Parameter{
		{Name: "TOKEN", Required: true, Text: &store.TextParameter{}},
		{Name: "MODEL", Choice: &store.ChoiceParameter{Choices: []string{"small", "large"}, DefaultValue: "small"}},
		{Name: "PORT", Text: &store.TextParameter{DefaultValue: "8080"}},
	}

	bindings, err := resolveParameterBindings(
		t.Context(),
		parameterBindingArgs{
			parameters: parameters,
			values:     map[string]string{"MODEL": "large"},
			secrets:    map[string]store.ManagedSecretReference{"TOKEN": {SecretID: "secret-1"}},
			resolver:   resolver,
		},
	)

	require.NoError(t, err)
	require.Len(t, bindings, 3)
	assert.Equal(t, &store.ManagedSecretReference{SecretID: "secret-1", VersionID: "version-9"}, bindings[0].ManagedSecret)
	assert.Equal(t, store.ParameterBinding{Name: "MODEL", Value: "large"}, bindings[1])
	assert.Equal(t, store.ParameterBinding{Name: "PORT", Value: "8080"}, bindings[2])
}

func TestResolveParameterBindingsRejectsInvalidSecretBindings(t *testing.T) {
	parameters := []store.Parameter{
		{Name: "TOKEN", Required: true, Text: &store.TextParameter{}},
		{Name: "MODEL", Required: true, Choice: &store.ChoiceParameter{Choices: []string{"small"}}},
	}

	_, err := resolveParameterBindings(
		t.Context(),
		parameterBindingArgs{
			parameters: parameters,
			values:     map[string]string{"TOKEN": "direct"},
			secrets: map[string]store.ManagedSecretReference{
				"TOKEN": {SecretID: "secret-1", VersionID: "version-1"},
				"MODEL": {SecretID: "secret-2", VersionID: "version-1"},
			},
			resolver: &fakeSecretResolver{},
		},
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, `parameter "TOKEN" cannot use both`)
	assert.ErrorContains(t, err, `choice parameter "MODEL" cannot be bound to a secret`)
}

func TestManagedSecretBindingUsesUICompatibleJSON(t *testing.T) {
	payload := store.LaunchableConfig{
		ID: "env-abc",
		ParameterBindings: []store.ParameterBinding{{
			Name: "TOKEN",
			ManagedSecret: &store.ManagedSecretReference{
				SecretID:  "secret-1",
				VersionID: "version-2",
			},
		}},
	}

	contents, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"id":"env-abc",
		"parameterBindings":[{
			"name":"TOKEN",
			"managedSecret":{"secretId":"secret-1","versionId":"version-2"}
		}]
	}`, string(contents))
	assert.NotContains(t, string(contents), `"value"`)
}

func TestRemoteLaunchPassesManagedSecretBindingToCreate(t *testing.T) {
	launchStore := &fakeLaunchStore{}
	info := &store.LaunchableResponse{
		Name: "remote-launch",
		CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
			InstanceType: "gpu.test",
			CloudCredID:  "cloud-cred-1",
		},
		BuildRequest: store.LaunchableBuildRequest{VMBuild: &store.VMBuild{}},
	}
	bindings := []store.ParameterBinding{{
		Name:          "TOKEN",
		ManagedSecret: &store.ManagedSecretReference{SecretID: "secret-1", VersionID: "version-2"},
	}}

	err := runRemoteLaunch(remoteLaunchArgs{
		terminal:     terminal.New(),
		store:        launchStore,
		launchableID: "env-abc",
		info:         info,
		bindings:     bindings,
		name:         "remote-launch",
		options: commandOptions{
			detached: true,
			timeout:  defaultLaunchTimeoutSeconds,
		},
	})

	require.NoError(t, err)
	require.Len(t, launchStore.created, 1)
	require.NotNil(t, launchStore.created[0].LaunchableConfig)
	assert.Equal(t, "env-abc", launchStore.created[0].LaunchableConfig.ID)
	assert.Equal(t, bindings, launchStore.created[0].LaunchableConfig.ParameterBindings)
}

func TestRemoteLaunchDoesNotFetchStartupScript(t *testing.T) {
	launchStore := &fakeLaunchStore{launchable: &store.LaunchableResponse{
		Name: "remote-launch",
		CreateWorkspaceRequest: store.LaunchableWorkspaceRequest{
			InstanceType: "gpu.test",
			CloudCredID:  "cloud-cred-1",
		},
		BuildRequest: store.LaunchableBuildRequest{VMBuild: &store.VMBuild{
			LifeCycleScriptAttr: &store.LifeCycleScriptAttr{ID: "script-1"},
		}},
	}}
	cmd := newCmdLaunch(terminal.New(), launchStore, launchDeps{secrets: &fakeSecretResolver{}})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"env-abc", "--detached"})

	err := cmd.Execute()

	require.NoError(t, err)
	require.Len(t, launchStore.created, 1)
	assert.Empty(t, launchStore.lifecycleCalls)
}

func TestFetchStartupScriptDoesNotMutateLaunchable(t *testing.T) {
	launchStore := &fakeLaunchStore{lifecycle: &store.LifeCycleScriptResponse{
		Attrs: &store.LifeCycleScriptAttr{Script: "echo hello"},
	}}
	info := &store.LaunchableResponse{BuildRequest: store.LaunchableBuildRequest{
		VMBuild: &store.VMBuild{LifeCycleScriptAttr: &store.LifeCycleScriptAttr{ID: "script-1", Script: "original"}},
	}}

	startupScript, err := fetchStartupScript(launchStore, "env-abc", info)

	require.NoError(t, err)
	assert.Equal(t, "echo hello", startupScript.Script)
	assert.Equal(t, "original", info.BuildRequest.VMBuild.LifeCycleScriptAttr.Script)
	assert.Equal(t, [][2]string{{"env-abc", "script-1"}}, launchStore.lifecycleCalls)
}

func TestRunLocalComposeFetchesYAMLAndKeepsSecretsOutOfArguments(t *testing.T) { //nolint:funlen // end-to-end local compose behavior
	runner := &fakeCommandRunner{paths: map[string]string{"docker": "/usr/bin/docker"}}
	fetcher := &fakeComposeFetcher{contents: []byte("services:\n  app:\n    image: example/app\n")}
	resolver := &fakeSecretResolver{values: map[string]string{"secret-1@version-2": "super-secret"}}
	info := &store.LaunchableResponse{
		BuildRequest: store.LaunchableBuildRequest{
			DockerCompose: &store.DockerCompose{
				FileURL:              "https://example.com/docker-compose.yaml",
				YamlString:           "ignored: true",
				EnvironmentVariables: map[string]string{"PUBLIC_SETTING": "enabled"},
				Registries: []*store.Registry{{
					Username: "registry-user",
					Password: "registry-password",
					Url:      "registry.example.com",
				}},
			},
		},
	}
	bindings := []store.ParameterBinding{{
		Name:          "API_TOKEN",
		ManagedSecret: &store.ManagedSecretReference{SecretID: "secret-1", VersionID: "version-2"},
	}}

	err := runLocalLaunchable(t.Context(), localLaunchArgs{
		terminal:     terminal.New(),
		launchableID: "env-abc",
		info:         info,
		bindings:     bindings,
		options: localOptions{
			name:     "My Launchable",
			detached: true,
			stdin:    bytes.NewReader(nil),
			stdout:   io.Discard,
			stderr:   io.Discard,
		},
		deps: launchDeps{runner: runner, fetchCompose: fetcher.Fetch, secrets: resolver},
	})

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/docker-compose.yaml", fetcher.url)
	require.Len(t, runner.commands, 2)
	assert.Equal(t, "registry-password\n", runner.commands[0].stdin)
	assert.NotContains(t, strings.Join(runner.commands[0].spec.args, " "), "registry-password")

	compose := runner.commands[1]
	t.Cleanup(func() { _ = os.RemoveAll(compose.spec.dir) })
	assert.Equal(t, fetcher.contents, []byte(compose.composeYAML))
	assert.Contains(t, compose.spec.args, "--detach")
	assert.Contains(t, compose.spec.args, "my-launchable")
	assert.Contains(t, compose.spec.args, compose.spec.dir)
	assert.Contains(t, compose.spec.args, filepath.Join(compose.spec.dir, "docker-compose.yaml"))
	assert.NotContains(t, strings.Join(compose.spec.args, " "), "super-secret")
	assert.Contains(t, compose.spec.env, "API_TOKEN=super-secret")
	assert.Contains(t, compose.spec.env, "PUBLIC_SETTING=enabled")
}

func TestRunLocalContainerPassesParameterNamesThroughDockerEnvironment(t *testing.T) {
	runner := &fakeCommandRunner{paths: map[string]string{"docker": "/usr/bin/docker"}}
	info := &store.LaunchableResponse{BuildRequest: store.LaunchableBuildRequest{
		CustomContainer: &store.CustomContainer{ContainerURL: "example/app:latest", EntryPoint: "python -m server"},
		Ports:           []store.LaunchablePort{{Port: "8080"}},
	}}
	bindings := []store.ParameterBinding{{Name: "TOKEN", Value: "secret-ish-direct-value"}}

	err := runLocalLaunchable(t.Context(), localLaunchArgs{
		terminal:     terminal.New(),
		launchableID: "env-abc",
		info:         info,
		bindings:     bindings,
		options: localOptions{
			name:   "container-test",
			stdout: io.Discard,
			stderr: io.Discard,
		},
		deps: launchDeps{runner: runner, secrets: &fakeSecretResolver{}},
	})

	require.NoError(t, err)
	require.Len(t, runner.commands, 1)
	command := runner.commands[0].spec
	t.Cleanup(func() { _ = os.RemoveAll(command.dir) })
	assert.Contains(t, command.args, "TOKEN")
	assert.Contains(t, command.args, "8080:8080")
	entrypointIndex := slices.Index(command.args, "--entrypoint")
	require.NotEqual(t, -1, entrypointIndex)
	assert.Equal(t, []string{"--entrypoint", "python", "example/app:latest", "-m", "server"}, command.args[entrypointIndex:])
	assert.Contains(t, command.args, command.dir+":/workspace")
	assert.Contains(t, command.args, "/workspace")
	assert.NotContains(t, strings.Join(command.args, " "), "secret-ish-direct-value")
	assert.Contains(t, command.env, "TOKEN=secret-ish-direct-value")
}

func TestRunLocalVMRequiresConfirmation(t *testing.T) {
	info := &store.LaunchableResponse{BuildRequest: store.LaunchableBuildRequest{
		VMBuild: &store.VMBuild{LifeCycleScriptAttr: &store.LifeCycleScriptAttr{Script: "echo hello"}},
	}}

	t.Run("canceled", func(t *testing.T) {
		runner := &fakeCommandRunner{paths: map[string]string{"bash": "/bin/bash"}}
		confirmer := &fakeConfirmer{result: false}
		err := runLocalLaunchable(t.Context(), localLaunchArgs{
			terminal:      terminal.New(),
			launchableID:  "env-abc",
			info:          info,
			startupScript: info.BuildRequest.VMBuild.LifeCycleScriptAttr,
			options: localOptions{
				name: "vm-test", stdout: io.Discard, stderr: io.Discard,
			},
			deps: launchDeps{runner: runner, confirm: confirmer.ConfirmYesNo, secrets: &fakeSecretResolver{}},
		})
		require.NoError(t, err)
		assert.Equal(t, 1, confirmer.calls)
		assert.Empty(t, runner.commands)
	})

	t.Run("approved by flag", func(t *testing.T) {
		runner := &fakeCommandRunner{paths: map[string]string{"bash": "/bin/bash"}}
		confirmer := &fakeConfirmer{result: false}
		err := runLocalLaunchable(t.Context(), localLaunchArgs{
			terminal:      terminal.New(),
			launchableID:  "env-abc",
			info:          info,
			startupScript: info.BuildRequest.VMBuild.LifeCycleScriptAttr,
			options: localOptions{
				name: "vm-test", approve: true, stdout: io.Discard, stderr: io.Discard,
			},
			deps: launchDeps{runner: runner, confirm: confirmer.ConfirmYesNo, secrets: &fakeSecretResolver{}},
		})
		require.NoError(t, err)
		assert.Zero(t, confirmer.calls)
		require.Len(t, runner.commands, 1)
		t.Cleanup(func() { _ = os.RemoveAll(runner.commands[0].spec.dir) })
		assert.Equal(t, []string{"-c", "echo hello"}, runner.commands[0].spec.args)
	})
}

func TestPrepareLocalWorkspaceClonesRepositoryIntoRequestedDirectory(t *testing.T) {
	runner := &fakeCommandRunner{paths: map[string]string{"git": "/usr/bin/git"}}
	workspace, err := prepareLocalWorkspace(t.Context(), localWorkspaceArgs{
		terminal:     terminal.New(),
		launchableID: "env-abc",
		repository: &store.LaunchableFile{
			URL:  "https://github.com/example/project.git",
			Path: "./source",
		},
		options: localOptions{stdout: io.Discard, stderr: io.Discard},
		runner:  runner,
	})

	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(workspace) })
	require.Len(t, runner.commands, 1)
	assert.Equal(t, "/usr/bin/git", runner.commands[0].spec.name)
	assert.Equal(t, filepath.Join(workspace, "source"), runner.commands[0].spec.dir)
	assert.Equal(t, []string{
		"clone",
		"https://github.com/example/project.git",
		filepath.Join(workspace, "source", "project"),
	}, runner.commands[0].spec.args)
}

func TestDetectBuildModeRejectsAmbiguousLaunchable(t *testing.T) {
	_, err := detectBuildMode(&store.LaunchableResponse{BuildRequest: store.LaunchableBuildRequest{
		VMBuild:       &store.VMBuild{},
		DockerCompose: &store.DockerCompose{YamlString: "services: {}"},
	}})
	assert.ErrorContains(t, err, "multiple build modes")
}

func TestParseLaunchableID(t *testing.T) {
	id, err := parseLaunchableID("https://console.brev.dev/launchable/deploy?launchableID=env-abc")
	require.NoError(t, err)
	assert.Equal(t, "env-abc", id)

	id, err = parseLaunchableID(" env-abc ")
	require.NoError(t, err)
	assert.Equal(t, "env-abc", id)

	_, err = parseLaunchableID("https://console.brev.dev/launchable/deploy?launchableID=env-abc/../../other")
	assert.Error(t, err)
}

func TestRemoteInstanceTypes(t *testing.T) {
	types, err := remoteInstanceTypes("a100.large, h100.large", "ignored")
	require.NoError(t, err)
	assert.Equal(t, "a100.large", types[0].Type)
	assert.Equal(t, "h100.large", types[1].Type)

	types, err = remoteInstanceTypes("", " gpu.test ")
	require.NoError(t, err)
	assert.Equal(t, "gpu.test", types[0].Type)

	_, err = remoteInstanceTypes("", "")
	assert.ErrorContains(t, err, "provide --type")
}
