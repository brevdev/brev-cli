package launch

import (
	"context"
	"fmt"
	"sort"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type containerLaunchArgs struct {
	terminal       *terminal.Terminal
	build          *store.CustomContainer
	ports          []store.LaunchablePort
	parameterNames []string
	env            []string
	workspace      string
	options        localOptions
	deps           launchDeps
}

type dockerLoginArgs struct {
	docker   string
	registry *store.Registry
	options  localOptions
	runner   commandRunner
}

func runContainer(ctx context.Context, args containerLaunchArgs) error {
	if strings.TrimSpace(args.build.ContainerURL) == "" {
		return breverrors.NewValidationError("container launchable has no image configured")
	}
	docker, err := localDocker(args.deps.runner)
	if err != nil {
		return err
	}
	if err := dockerLogin(ctx, dockerLoginArgs{
		docker:   docker,
		registry: args.build.Registry,
		options:  args.options,
		runner:   args.deps.runner,
	}); err != nil {
		return err
	}
	dockerArgs := []string{"run", "--name", safeLocalName(args.options.name)}
	if args.options.detached {
		dockerArgs = append(dockerArgs, "--detach")
	} else {
		dockerArgs = append(dockerArgs, "--rm")
	}
	for _, name := range args.parameterNames {
		dockerArgs = append(dockerArgs, "--env", name)
	}
	for _, port := range portMappings(args.ports) {
		dockerArgs = append(dockerArgs, "--publish", port)
	}
	dockerArgs = append(dockerArgs, "--volume", args.workspace+":/workspace", "--workdir", "/workspace")
	if entrypoint := strings.TrimSpace(args.build.EntryPoint); entrypoint != "" {
		dockerArgs = append(dockerArgs, "--entrypoint", entrypoint)
	}
	dockerArgs = append(dockerArgs, args.build.ContainerURL)

	args.terminal.Vprintf("Starting %q with Docker.\n", args.options.name)
	if err := args.deps.runner.Run(ctx, commandSpec{
		name:   docker,
		args:   dockerArgs,
		dir:    args.workspace,
		env:    args.env,
		stdin:  args.options.stdin,
		stdout: args.options.stdout,
		stderr: args.options.stderr,
	}); err != nil {
		return fmt.Errorf("run launchable container: %w", err)
	}
	return nil
}

func dockerLogin(ctx context.Context, args dockerLoginArgs) error {
	if args.registry == nil || args.registry.Username == "" || args.registry.Password == "" {
		return nil
	}
	dockerArgs := []string{"login"}
	if args.registry.Url != "" {
		dockerArgs = append(dockerArgs, args.registry.Url)
	}
	dockerArgs = append(dockerArgs, "--username", args.registry.Username, "--password-stdin")
	if err := args.runner.Run(ctx, commandSpec{
		name:   args.docker,
		args:   dockerArgs,
		stdin:  strings.NewReader(args.registry.Password + "\n"),
		stdout: args.options.stdout,
		stderr: args.options.stderr,
	}); err != nil {
		return fmt.Errorf("log in to Docker registry %q: %w", args.registry.Url, err)
	}
	return nil
}

func localDocker(runner commandRunner) (string, error) {
	docker, err := runner.LookPath("docker")
	if err != nil {
		return "", breverrors.NewValidationError("local container and Docker Compose launchables require Docker on PATH")
	}
	return docker, nil
}

func portMappings(ports []store.LaunchablePort) []string {
	result := make([]string, 0, len(ports))
	for _, port := range ports {
		value := strings.TrimSpace(port.Port)
		if value != "" {
			result = append(result, value+":"+value)
		}
	}
	sort.Strings(result)
	return result
}
