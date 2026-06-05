package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/briandowns/spinner"
)

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
	err = WaitForSSHToBeAvailable("slow-host", s)
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
