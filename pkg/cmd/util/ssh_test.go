package util

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/briandowns/spinner"
	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func TestRequireRunning(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		wantErr    bool
		wantSubstr []string
	}{
		{
			name:   "running is ready",
			status: entity.Running,
		},
		{
			name:       "stopped tells the user how to start it",
			status:     entity.Stopped,
			wantErr:    true,
			wantSubstr: []string{"is not running", "please start it with: brev start my-box"},
		},
		{
			name:       "mid-boot names the status instead of suggesting start",
			status:     entity.Starting,
			wantErr:    true,
			wantSubstr: []string{"is not running (status: STARTING)", "brev ls"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequireRunning(&entity.Workspace{Name: "my-box", Status: tt.status})

			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, substr := range tt.wantSubstr {
				assert.Contains(t, err.Error(), substr)
			}
		})
	}
}

func TestWaitForSSHToBeAvailable_StopsAfterMaxAttempts(t *testing.T) {
	dir := t.TempDir()
	attemptsFile := filepath.Join(dir, "attempts")
	fakeSSH := filepath.Join(dir, "ssh")
	script := "#!/bin/sh\n" +
		"n=$(cat " + attemptsFile + " 2>/dev/null || echo 0)\n" +
		"echo $((n+1)) > " + attemptsFile + "\n" +
		"echo 'ssh: connect to host my-box port 22: Connection refused' >&2\n" +
		"exit 255\n"
	require.NoError(t, os.WriteFile(fakeSSH, []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	originalSleep := sshAvailabilityRetrySleep
	sshAvailabilityRetrySleep = 0
	t.Cleanup(func() { sshAvailabilityRetrySleep = originalSleep })

	err := WaitForSSHToBeAvailable("my-box", spinner.New(spinner.CharSets[9], 100*time.Millisecond), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "was not available after 3 attempts")
	assert.IsType(t, breverrors.ValidationError{}, pkgerrors.Cause(err), "expected failures must render without a stack trace")

	raw, readErr := os.ReadFile(attemptsFile)
	require.NoError(t, readErr)
	assert.Equal(t, "3", strings.TrimSpace(string(raw)), "transient failures must stop after the attempt budget")
}

func TestWaitForSSHToBeAvailable_RefreshesConfigWhenAliasIsMissing(t *testing.T) {
	dir := t.TempDir()
	attemptsFile := filepath.Join(dir, "attempts")
	refreshesFile := filepath.Join(dir, "refreshes")
	fakeSSH := filepath.Join(dir, "ssh")
	// The alias "appears" once the refresh callback has run twice.
	script := "#!/bin/sh\n" +
		"n=$(cat " + attemptsFile + " 2>/dev/null || echo 0)\n" +
		"echo $((n+1)) > " + attemptsFile + "\n" +
		"r=$(cat " + refreshesFile + " 2>/dev/null || echo 0)\n" +
		"if [ \"$r\" -ge 2 ]; then echo connected; exit 0; fi\n" +
		"echo 'ssh: Could not resolve hostname my-box: nodename nor servname provided, or not known' >&2\n" +
		"exit 255\n"
	require.NoError(t, os.WriteFile(fakeSSH, []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	originalSleep := sshAvailabilityRetrySleep
	sshAvailabilityRetrySleep = 0
	t.Cleanup(func() { sshAvailabilityRetrySleep = originalSleep })

	refreshes := 0
	refreshConfig := func() error {
		refreshes++
		return os.WriteFile(refreshesFile, []byte(fmt.Sprintf("%d", refreshes)), 0o644)
	}

	err := WaitForSSHToBeAvailable("my-box", spinner.New(spinner.CharSets[9], 100*time.Millisecond), refreshConfig)

	require.NoError(t, err, "the alias appearing after a config refresh must be picked up")
	assert.GreaterOrEqual(t, refreshes, 2, "each unresolved-alias failure must regenerate the config")
}

// If the endpoint never gets published, say so instead of reporting a network
// problem the user cannot act on.
func TestWaitForSSHToBeAvailable_AliasNeverAppearsReportsProvisioning(t *testing.T) {
	dir := t.TempDir()
	attemptsFile := filepath.Join(dir, "attempts")
	fakeSSH := filepath.Join(dir, "ssh")
	script := "#!/bin/sh\n" +
		"n=$(cat " + attemptsFile + " 2>/dev/null || echo 0)\n" +
		"echo $((n+1)) > " + attemptsFile + "\n" +
		"echo 'ssh: Could not resolve hostname my-box: nodename nor servname provided, or not known' >&2\n" +
		"exit 255\n"
	require.NoError(t, os.WriteFile(fakeSSH, []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	originalSleep := sshAvailabilityRetrySleep
	sshAvailabilityRetrySleep = 0
	t.Cleanup(func() { sshAvailabilityRetrySleep = originalSleep })

	refreshes := 0
	err := WaitForSSHToBeAvailable("my-box", spinner.New(spinner.CharSets[9], 100*time.Millisecond), func() error {
		refreshes++
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no SSH address for my-box yet")
	assert.Contains(t, err.Error(), "provisioning")
	assert.IsType(t, breverrors.ValidationError{}, pkgerrors.Cause(err), "expected failures must render without a stack trace")
	assert.Equal(t, 2, refreshes, "the config must be regenerated between attempts")

	raw, readErr := os.ReadFile(attemptsFile)
	require.NoError(t, readErr)
	assert.Equal(t, "3", strings.TrimSpace(string(raw)), "the alias budget is the same 3 attempts")
}

func TestWaitForSSHToBeAvailable_FailsFastOnPermanentError(t *testing.T) {
	dir := t.TempDir()
	attemptsFile := filepath.Join(dir, "attempts")
	fakeSSH := filepath.Join(dir, "ssh")
	script := "#!/bin/sh\n" +
		"n=$(cat " + attemptsFile + " 2>/dev/null || echo 0)\n" +
		"echo $((n+1)) > " + attemptsFile + "\n" +
		"echo 'Host key verification failed.' >&2\n" +
		"exit 255\n"
	require.NoError(t, os.WriteFile(fakeSSH, []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := WaitForSSHToBeAvailable("my-box", spinner.New(spinner.CharSets[9], 100*time.Millisecond), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Host key verification failed")
	assert.IsType(t, breverrors.ValidationError{}, pkgerrors.Cause(err), "expected failures must render without a stack trace")

	raw, readErr := os.ReadFile(attemptsFile)
	require.NoError(t, readErr)
	assert.Equal(t, "1", strings.TrimSpace(string(raw)), "a permanent failure must not be retried")
}

func TestWaitForSSHToBeAvailableTimesOutStuckSSHAttempt(t *testing.T) {
	dir := t.TempDir()
	fakeSSH := filepath.Join(dir, "ssh")
	err := os.WriteFile(fakeSSH, []byte("#!/bin/sh\nexec sleep 5\n"), 0o755)
	if err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	originalAttemptTimeout := sshAvailabilityAttemptTimeout
	originalWaitDelay := sshAvailabilityWaitDelay
	originalRetrySleep := sshAvailabilityRetrySleep
	originalMaxAttempts := sshAvailabilityMaxAttempts
	sshAvailabilityAttemptTimeout = 50 * time.Millisecond
	sshAvailabilityWaitDelay = 50 * time.Millisecond
	sshAvailabilityRetrySleep = 0
	sshAvailabilityMaxAttempts = 0
	t.Cleanup(func() {
		sshAvailabilityAttemptTimeout = originalAttemptTimeout
		sshAvailabilityWaitDelay = originalWaitDelay
		sshAvailabilityRetrySleep = originalRetrySleep
		sshAvailabilityMaxAttempts = originalMaxAttempts
	})

	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	start := time.Now()
	err = WaitForSSHToBeAvailable("slow-host", s, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected stuck ssh attempt to fail")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("expected stuck ssh attempt to be killed quickly, took %v", elapsed)
	}
	if !strings.Contains(err.Error(), "timed out after") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

// captureStderr runs fn while capturing stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = writePipe
	defer func() { os.Stderr = original }()

	fn()

	require.NoError(t, writePipe.Close())
	out, err := io.ReadAll(readPipe)
	require.NoError(t, err)
	return string(out)
}

func TestWaitForSSHToBeAvailable_FailuresRenderWithoutStackFrames(t *testing.T) {
	viper.Set("feature.debug", true)
	t.Cleanup(viper.Reset)

	control := captureStderr(t, func() {
		cmderrors.DisplayAndHandleError(breverrors.WrapAndTrace(errors.New("unexpected boom")))
	})
	require.Contains(t, control, "ssh_test.go", "control must reproduce the stack-frame dump")

	dir := t.TempDir()
	fakeSSH := filepath.Join(dir, "ssh")
	script := "#!/bin/sh\n" +
		"echo 'ssh: Could not resolve hostname my-box: nodename nor servname provided, or not known' >&2\n" +
		"exit 255\n"
	require.NoError(t, os.WriteFile(fakeSSH, []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	originalSleep := sshAvailabilityRetrySleep
	sshAvailabilityRetrySleep = 0
	t.Cleanup(func() { sshAvailabilityRetrySleep = originalSleep })

	err := WaitForSSHToBeAvailable("my-box", spinner.New(spinner.CharSets[9], 100*time.Millisecond), func() error { return nil })
	require.Error(t, err)

	// Callers wrap the error before it reaches the renderer; the type must
	// survive that wrapping.
	out := captureStderr(t, func() { cmderrors.DisplayAndHandleError(breverrors.WrapAndTrace(err)) })
	assert.Contains(t, out, "no SSH address for my-box yet")
	assert.NotContains(t, out, ".go:", "expected failures must not print stack frames")
}
