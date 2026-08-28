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

	// If the launchable has no repository, return the workspace immediately
	if args.repository == nil {
		return workspace, nil
	}

	// If the launchable has a repository, clone it into the workspace
	directory := filepath.Join(workspace, args.repository.Path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create launchable repository directory: %w", err)
	}
	git, err := args.runner.LookPath("git")
	if err != nil {
		return "", breverrors.NewValidationError("repository-backed launchables require git on PATH")
	}
	destination := filepath.Join(directory, repositoryName(args.repository.URL))
	if err := args.runner.Run(ctx, commandSpec{
		name:   git,
		args:   []string{"clone", args.repository.URL, destination},
		dir:    directory,
		stdin:  args.options.stdin,
		stdout: args.options.stdout,
		stderr: args.options.stderr,
	}); err != nil {
		return "", fmt.Errorf("clone launchable repository: %w", err)
	}
	return workspace, nil
}

func repositoryName(sourceURL string) string {
	return strings.TrimSuffix(path.Base(strings.TrimRight(sourceURL, "/")), ".git")
}
