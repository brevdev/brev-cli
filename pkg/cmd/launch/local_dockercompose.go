package launch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

const maxComposeFileSize = 10 << 20

type composeLaunchArgs struct {
	terminal        *terminal.Terminal
	build           *store.DockerCompose
	launchableID    string
	parameterValues map[string]string
	options         localOptions
	deps            launchDeps
}

type composeFileFetcher func(ctx context.Context, url string) ([]byte, error)

func runCompose(ctx context.Context, args composeLaunchArgs) error {
	docker, err := localDocker(args.deps.runner)
	if err != nil {
		return err
	}
	contents, err := composeContents(ctx, args.build, args.deps.fetchCompose)
	if err != nil {
		return err
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get local working directory: %w", err)
	}
	tempDir, err := os.MkdirTemp("", "brev-launchable-"+safeLocalName(args.launchableID)+"-")
	if err != nil {
		return fmt.Errorf("create temporary launchable directory: %w", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // best-effort cleanup
	composePath := filepath.Join(tempDir, "docker-compose.yaml")
	if err := os.WriteFile(composePath, contents, 0o600); err != nil {
		return fmt.Errorf("write local compose file: %w", err)
	}
	for _, registry := range args.build.Registries {
		if err := dockerLogin(ctx, dockerLoginArgs{
			docker:   docker,
			registry: registry,
			options:  args.options,
			runner:   args.deps.runner,
		}); err != nil {
			return err
		}
	}
	env := mergeEnvironment(os.Environ(), args.build.EnvironmentVariables, args.parameterValues)
	dockerArgs := []string{
		"compose",
		"--project-name", safeLocalName(args.options.name),
		"--project-directory", workingDir,
		"--file", composePath,
		"up",
	}
	if args.options.detached {
		dockerArgs = append(dockerArgs, "--detach")
	}

	args.terminal.Vprintf("Starting %q with Docker Compose.\n", args.options.name)
	if err := args.deps.runner.Run(ctx, commandSpec{
		name:   docker,
		args:   dockerArgs,
		dir:    workingDir,
		env:    env,
		stdin:  args.options.stdin,
		stdout: args.options.stdout,
		stderr: args.options.stderr,
	}); err != nil {
		return fmt.Errorf("run Docker Compose launchable: %w", err)
	}
	args.terminal.Vprintf("Local launchable %q started successfully.\n", args.options.name)
	return nil
}

func composeContents(ctx context.Context, build *store.DockerCompose, fetch composeFileFetcher) ([]byte, error) {
	if strings.TrimSpace(build.FileURL) != "" {
		contents, err := fetch(ctx, build.FileURL)
		if err != nil {
			return nil, fmt.Errorf("fetch launchable Docker Compose file: %w", err)
		}
		return contents, nil
	}
	if strings.TrimSpace(build.YamlString) == "" {
		return nil, breverrors.NewValidationError("Docker Compose launchable has no file URL or YAML content")
	}
	return []byte(build.YamlString), nil
}

func fetchComposeFile(ctx context.Context, sourceURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create compose request: %w", err)
	}
	client := http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download compose file: %w", err)
	}
	defer response.Body.Close() //nolint:errcheck // body is read before returning
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download compose file: server returned %s", response.Status)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maxComposeFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}
	if len(contents) > maxComposeFileSize {
		return nil, fmt.Errorf("compose file exceeds %d MiB", maxComposeFileSize>>20)
	}
	return contents, nil
}
