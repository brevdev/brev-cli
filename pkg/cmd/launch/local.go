package launch

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type localOptions struct {
	name     string
	detached bool
	approve  bool
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
}

type localLaunchArgs struct {
	terminal      *terminal.Terminal
	launchableID  string
	info          *store.LaunchableResponse
	startupScript *store.LifeCycleScriptAttr
	bindings      []store.ParameterBinding
	options       localOptions
	deps          launchDeps
}

type launchDeps struct {
	runner       commandRunner
	fetchCompose composeFileFetcher
	confirm      confirmFunc
	secrets      managedSecretResolver
}

type commandSpec struct {
	name   string
	args   []string
	dir    string
	env    []string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type commandRunner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, spec commandSpec) error
}

type execCommandRunner struct{}

type localBuildMode string

const (
	buildModeVM        localBuildMode = "VM"
	buildModeContainer localBuildMode = "container"
	buildModeCompose   localBuildMode = "Docker Compose"
	buildModeVerb      localBuildMode = "Verb"
)

func defaultLaunchDeps(launchStore Store) launchDeps {
	return launchDeps{
		runner:       execCommandRunner{},
		fetchCompose: fetchComposeFile,
		confirm:      confirmStartupScript,
		secrets:      newDevplaneManagedSecretResolver(launchStore),
	}
}

func runLocalLaunchable(ctx context.Context, args localLaunchArgs) error {
	mode, err := detectBuildMode(args.info)
	if err != nil {
		return err
	}
	if mode == buildModeVerb {
		return breverrors.NewValidationError("Verb launchables cannot yet run locally")
	}
	parameterValues, err := localParameterValues(ctx, args.bindings, args.deps.secrets)
	if err != nil {
		return err
	}
	workspace, err := prepareLocalWorkspace(ctx, localWorkspaceArgs{
		terminal:     args.terminal,
		launchableID: args.launchableID,
		file:         args.info.File,
		options:      args.options,
		runner:       args.deps.runner,
	})
	if err != nil {
		return err
	}

	switch mode {
	case buildModeVM:
		return runVM(ctx, vmLaunchArgs{
			terminal:  args.terminal,
			script:    args.startupScript,
			env:       mergeEnvironment(os.Environ(), parameterValues),
			workspace: workspace,
			options:   args.options,
			deps:      args.deps,
		})
	case buildModeContainer:
		return runContainer(ctx, containerLaunchArgs{
			terminal:       args.terminal,
			build:          args.info.BuildRequest.CustomContainer,
			ports:          args.info.BuildRequest.Ports,
			parameterNames: sortedKeys(parameterValues),
			env:            mergeEnvironment(os.Environ(), parameterValues),
			workspace:      workspace,
			options:        args.options,
			deps:           args.deps,
		})
	case buildModeCompose:
		return runCompose(ctx, composeLaunchArgs{
			terminal:        args.terminal,
			build:           args.info.BuildRequest.DockerCompose,
			parameterValues: parameterValues,
			workspace:       workspace,
			options:         args.options,
			deps:            args.deps,
		})
	default:
		return breverrors.NewValidationError(fmt.Sprintf("unsupported local build mode %q", mode))
	}
}

func detectBuildMode(info *store.LaunchableResponse) (localBuildMode, error) {
	if info == nil {
		return "", breverrors.NewValidationError("launchable configuration is missing")
	}
	var modes []localBuildMode
	if info.BuildRequest.VMBuild != nil {
		modes = append(modes, buildModeVM)
	}
	if info.BuildRequest.CustomContainer != nil {
		modes = append(modes, buildModeContainer)
	}
	if info.BuildRequest.DockerCompose != nil {
		modes = append(modes, buildModeCompose)
	}
	if info.BuildRequest.VerbBuild != nil {
		modes = append(modes, buildModeVerb)
	}
	if len(modes) == 0 {
		return "", breverrors.NewValidationError("launchable does not define a supported build mode")
	}
	if len(modes) > 1 {
		return "", breverrors.NewValidationError("launchable defines multiple build modes; cannot choose a safe local build")
	}
	return modes[0], nil
}

func localParameterValues(ctx context.Context, bindings []store.ParameterBinding, resolver managedSecretResolver) (map[string]string, error) {
	values := make(map[string]string, len(bindings))
	for _, binding := range bindings {
		if binding.ManagedSecret == nil {
			values[binding.Name] = binding.Value
			continue
		}
		if resolver == nil {
			return nil, fmt.Errorf("managed-secret resolver is not configured")
		}
		value, err := resolver.GetValue(ctx, *binding.ManagedSecret)
		if err != nil {
			return nil, fmt.Errorf("resolve managed secret for parameter %q: %w", binding.Name, err)
		}
		values[binding.Name] = value
	}
	return values, nil
}

func mergeEnvironment(base []string, overrides ...map[string]string) []string {
	values := make(map[string]string, len(base))
	for _, item := range base {
		name, value, ok := strings.Cut(item, "=")
		if ok {
			values[name] = value
		}
	}
	for _, items := range overrides {
		for name, value := range items {
			values[name] = value
		}
	}
	names := sortedKeys(values)
	env := make([]string, 0, len(names))
	for _, name := range names {
		env = append(env, name+"="+values[name])
	}
	return env
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func safeLocalName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var result strings.Builder
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9', char == '-', char == '_':
			result.WriteRune(char)
		default:
			result.WriteByte('-')
		}
	}
	value := strings.Trim(result.String(), "-_")
	if value == "" {
		return "brev-launchable"
	}
	return value
}

func (execCommandRunner) LookPath(file string) (string, error) {
	path, err := exec.LookPath(file)
	if err != nil {
		return "", fmt.Errorf("find %s executable: %w", file, err)
	}
	return path, nil
}

func (execCommandRunner) Run(ctx context.Context, spec commandSpec) error {
	cmd := exec.CommandContext(ctx, spec.name, spec.args...) //nolint:gosec // commands use fixed executables and flags
	cmd.Dir = spec.dir
	cmd.Env = spec.env
	cmd.Stdin = spec.stdin
	cmd.Stdout = spec.stdout
	cmd.Stderr = spec.stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", spec.name, err)
	}
	return nil
}
