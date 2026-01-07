package spark

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
)

// RemoteRunner executes commands on a Spark host over ssh and returns stdout/stderr.
type RemoteRunner struct {
	fs afero.Fs
}

func NewRemoteRunner(fs afero.Fs) RemoteRunner {
	return RemoteRunner{fs: fs}
}

// Run executes the provided remote shell command via ssh and returns combined output.
func (r RemoteRunner) Run(ctx context.Context, host Host, remoteCmd string) (string, error) {
	if host.IdentityFile == "" {
		return "", fmt.Errorf("missing identity file for %s", host.Alias)
	}

	exists, err := afero.Exists(r.fs, host.IdentityFile)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("identity file not found at %s", host.IdentityFile)
	}

	argv := buildSSHCommand(host, remoteCmd)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("ssh to %s failed for command %q: %w\noutput:\n%s", hostLabel(host), remoteCmd, err, buf.String())
	}
	return buf.String(), nil
}

func buildSSHCommand(host Host, remoteCmd string) []string {
	args := []string{
		"ssh",
		"-i", host.IdentityFile,
		"-p", strconv.Itoa(host.Port),
		"-o", "BatchMode=yes",
	}

	if len(host.Options) > 0 {
		keys := make([]string, 0, len(host.Options))
		for k := range host.Options {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			args = append(args, "-o", fmt.Sprintf("%s=%s", key, host.Options[key]))
		}
	}

	escaped := escapeSingleQuotes(remoteCmd)
	args = append(args, fmt.Sprintf("%s@%s", host.User, host.Hostname), "--", "bash", "-lc", fmt.Sprintf("'%s'", escaped))
	return args
}

// WithTimeout wraps a context with timeout; if parent already has deadline it respects the earlier one.
func WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

// QuoteForShell safely wraps a string for single-quoted shell contexts.
func QuoteForShell(s string) string {
	if s == "" {
		return "''"
	}
	// Replace single quotes with '"'"' sequence.
	return "'" + strings.ReplaceAll(s, "'", `'\"'\"'`) + "'"
}

func hostLabel(h Host) string {
	return fmt.Sprintf("%s@%s:%d", h.User, h.Hostname, h.Port)
}

// HostLabel is exported for reuse in other packages.
func HostLabel(h Host) string {
	return hostLabel(h)
}

// escapeSingleQuotes makes a string safe for single-quoted bash -lc payloads.
func escapeSingleQuotes(s string) string {
	if s == "" {
		return s
	}
	return strings.ReplaceAll(s, "'", `'\''`)
}
