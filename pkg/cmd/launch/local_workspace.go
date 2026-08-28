package launch

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type localWorkspaceArgs struct {
	terminal     *terminal.Terminal
	launchableID string
	repository   *store.LaunchableFile
	options      localOptions
	runner       commandRunner
}

func prepareLocalWorkspace(ctx context.Context, args localWorkspaceArgs) (string, error) {
	workspace, err := os.MkdirTemp("", "brev-launch-"+safeLocalName(args.launchableID)+"-")
	if err != nil {
		return "", fmt.Errorf("create local launchable workspace: %w", err)
	}
	args.terminal.Vprintf("Local workspace: %s\n", workspace)

	if args.repository == nil || strings.TrimSpace(args.repository.URL) == "" {
		return workspace, nil
	}
	directory := filepath.Join(workspace, args.repository.Path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create launchable repository directory: %w", err)
	}
	if err := cloneRepository(ctx, cloneRepositoryArgs{
		sourceURL: args.repository.URL,
		directory: directory,
		options:   args.options,
		runner:    args.runner,
	}); err != nil {
		return "", err
	}
	return workspace, nil
}

type cloneRepositoryArgs struct {
	sourceURL string
	directory string
	options   localOptions
	runner    commandRunner
}

func cloneRepository(ctx context.Context, args cloneRepositoryArgs) error {
	git, err := args.runner.LookPath("git")
	if err != nil {
		return breverrors.NewValidationError("repository-backed launchables require git on PATH")
	}
	destination := filepath.Join(args.directory, repositoryName(args.sourceURL))
	if err := args.runner.Run(ctx, commandSpec{
		name:   git,
		args:   []string{"clone", args.sourceURL, destination},
		dir:    args.directory,
		stdin:  args.options.stdin,
		stdout: args.options.stdout,
		stderr: args.options.stderr,
	}); err != nil {
		return fmt.Errorf("clone launchable repository: %w", err)
	}
	return nil
}

func repositoryName(sourceURL string) string {
	return strings.TrimSuffix(path.Base(strings.TrimRight(sourceURL, "/")), ".git")
}
