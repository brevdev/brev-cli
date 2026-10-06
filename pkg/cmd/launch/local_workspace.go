package launch

import (
	"context"
	"fmt"
	"net/url"
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
	file         *store.LaunchableFile
	options      localOptions
	runner       commandRunner
}

func prepareLocalWorkspace(ctx context.Context, args localWorkspaceArgs) (string, error) {
	workspace, err := os.MkdirTemp("", "brev-launch-"+safeLocalName(args.launchableID)+"-")
	if err != nil {
		return "", fmt.Errorf("create local launchable workspace: %w", err)
	}
	args.terminal.Vprintf("Local workspace: %s\n", workspace)

	if args.file == nil {
		return workspace, nil
	}

	directory := filepath.Join(workspace, args.file.Path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create launchable file directory: %w", err)
	}
	if fileName, ok := rawFileName(args.file.URL); ok {
		curl, err := args.runner.LookPath("curl")
		if err != nil {
			return "", breverrors.NewValidationError("file-backed launchables require curl on PATH")
		}
		if err := args.runner.Run(ctx, commandSpec{
			name:   curl,
			args:   []string{"--fail", "--location", "--output", filepath.Join(directory, fileName), args.file.URL},
			dir:    directory,
			stdin:  args.options.stdin,
			stdout: args.options.stdout,
			stderr: args.options.stderr,
		}); err != nil {
			return "", fmt.Errorf("download launchable file: %w", err)
		}
		return workspace, nil
	}
	git, err := args.runner.LookPath("git")
	if err != nil {
		return "", breverrors.NewValidationError("repository-backed launchables require git on PATH")
	}
	destination := filepath.Join(directory, repositoryName(args.file.URL))
	if err := args.runner.Run(ctx, commandSpec{
		name:   git,
		args:   []string{"clone", args.file.URL, destination},
		dir:    directory,
		stdin:  args.options.stdin,
		stdout: args.options.stdout,
		stderr: args.options.stderr,
	}); err != nil {
		return "", fmt.Errorf("clone launchable repository: %w", err)
	}
	return workspace, nil
}

func rawFileName(sourceURL string) (string, bool) {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return "", false
	}
	escapedPath := parsed.EscapedPath()
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	isRawFile := host == "gitlab.com" && strings.Contains(escapedPath, "/-/raw/")
	if host == "github.com" {
		parts := strings.Split(strings.Trim(escapedPath, "/"), "/")
		isRawFile = len(parts) >= 5 && parts[2] == "raw"
	}
	if !isRawFile {
		return "", false
	}
	fileName := path.Base(escapedPath)
	if fileName == "." || fileName == "/" {
		return "", false
	}
	return fileName, true
}

func repositoryName(sourceURL string) string {
	return strings.TrimSuffix(path.Base(strings.TrimRight(sourceURL, "/")), ".git")
}
