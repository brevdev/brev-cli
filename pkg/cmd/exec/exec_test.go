package exec

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSSHFailureAllowsReplayOnlyBeforeSession(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "remote exit", err: &sshAttemptError{exitCode: 1}, want: false},
		{name: "remote exit 127", err: &sshAttemptError{exitCode: 127}, want: false},
		{name: "exit 255 with empty diagnostic", err: &sshAttemptError{exitCode: 255}, want: false},
		{
			name: "session dropped",
			err:  &sshAttemptError{exitCode: 255, diag: "client_loop: send disconnect: Broken pipe\n"},
			want: false,
		},
		{
			name: "connection closed after the session",
			err:  &sshAttemptError{exitCode: 255, diag: "Connection closed by 10.0.0.8 port 22"},
			want: false,
		},
		{
			name: "connection reset after the session",
			err:  &sshAttemptError{exitCode: 255, diag: "Connection reset by peer"},
			want: false,
		},
		{
			name: "connection refused",
			err:  &sshAttemptError{exitCode: 255, diag: "ssh: connect to host gpu port 22: Connection refused"},
			want: true,
		},
		{
			name: "localized connect failure",
			err:  &sshAttemptError{exitCode: 255, diag: "ssh: connect to host gpu port 22: Verbindung abgelehnt"},
			want: true,
		},
		{
			name: "connect timeout",
			err:  &sshAttemptError{exitCode: 255, diag: "ssh: connect to host gpu port 22: Connection timed out"},
			want: true,
		},
		{
			name: "macos connect timeout",
			err:  &sshAttemptError{exitCode: 255, diag: "ssh: connect to host gpu port 22: Operation timed out"},
			want: true,
		},
		{
			name: "unresolved alias",
			err:  &sshAttemptError{exitCode: 255, diag: "ssh: Could not resolve hostname gpu: Name or service not known"},
			want: true,
		},
		{
			name: "handshake before the command",
			err:  &sshAttemptError{exitCode: 255, diag: "kex_exchange_identification: Connection closed by remote host"},
			want: true,
		},
		{name: "untyped error", err: fmt.Errorf("ssh: exit status 1"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sshFailureAllowsReplay(tc.err); got != tc.want {
				t.Fatalf("sshFailureAllowsReplay() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExecDoesNotReplayRemoteExit(t *testing.T) {
	calls, output, err, panicked := execWithSSHResult(t, &sshAttemptError{
		err:      fmt.Errorf("exit status 1"),
		exitCode: 1,
	})
	if calls != 1 {
		t.Fatalf("remote command ran %d times, want 1", calls)
	}
	if strings.Contains(output, "Connection failed") {
		t.Fatalf("remote exit was treated as a dead connection:\n%s", output)
	}
	if err == nil && panicked == nil {
		t.Fatal("expected the remote failure to be returned")
	}
}

func TestExecDoesNotReplaySessionDrop(t *testing.T) {
	calls, output, err, panicked := execWithSSHResult(t, &sshAttemptError{
		err:      fmt.Errorf("exit status 255"),
		exitCode: 255,
		diag:     "client_loop: send disconnect: Broken pipe\n",
	})
	if calls != 1 {
		t.Fatalf("remote command ran %d times, want 1", calls)
	}
	if strings.Contains(output, "Connection failed") {
		t.Fatalf("dropped session was replayed via SSH recovery:\n%s", output)
	}
	if err == nil && panicked == nil {
		t.Fatal("expected the ssh failure to be returned")
	}
}

func TestExecStillRecoversWhenSSHNeverConnected(t *testing.T) {
	_, output, _, _ := execWithSSHResult(t, &sshAttemptError{
		err:      fmt.Errorf("exit status 255"),
		exitCode: 255,
		diag:     "ssh: connect to host gpu port 22: Connection refused\n",
	})
	if !strings.Contains(output, "Connection failed, checking instance status...") {
		t.Fatalf("a refused connection should still enter recovery, output:\n%s", output)
	}
}

func TestRunSSHWithTimeoutKeepsPreSessionDiagnostic(t *testing.T) {
	sshPath := writeFakeSSH(t, `#!/bin/sh
prev=
for a in "$@"; do
  if [ "$prev" = "-E" ]; then
    printf '%s\n' 'ssh: connect to host inst port 22: Connection refused' > "$a"
    exit 255
  fi
  prev=$a
done
printf '%s\n' 'fake ssh: missing -E' >&2
exit 255
`)
	t.Setenv("SSH_AUTH_SOCK", "/tmp/brev-exec-test-sock")
	t.Setenv("PATH", filepath.Dir(sshPath)+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := runSSHWithTimeout("inst", "true", 1)
	if !sshFailureAllowsReplay(err) {
		t.Fatalf("connection refused was not classified as pre-session: %v", err)
	}
}

func TestRunSSHWithTimeoutDoesNotReplayRemoteStatus(t *testing.T) {
	sshPath := writeFakeSSH(t, `#!/bin/sh
prev=
for a in "$@"; do
  if [ "$prev" = "-E" ]; then
    : > "$a"
    exit 1
  fi
  prev=$a
done
exit 1
`)
	t.Setenv("SSH_AUTH_SOCK", "/tmp/brev-exec-test-sock")
	t.Setenv("PATH", filepath.Dir(sshPath)+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := runSSHWithTimeout("inst", "mhcflurry eval", 1)
	if sshFailureAllowsReplay(err) {
		t.Fatalf("remote exit 1 was classified as replayable: %v", err)
	}
	var attempt *sshAttemptError
	if !errorsAsAttempt(err, &attempt) || attempt.exitCode != 1 {
		t.Fatalf("exit code = %v, err = %v", attempt, err)
	}
}

func execWithSSHResult(t *testing.T, sshErr error) (calls int, output string, ret error, panicked any) {
	t.Helper()
	orig := runExecSSH
	t.Cleanup(func() { runExecSSH = orig })
	runExecSSH = func(alias, command string, connectTimeoutSecs int) error {
		calls++
		return sshErr
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = oldStderr }()

	func() {
		defer func() { panicked = recover() }()
		ret = runExecCommand(nil, nil, "gpu-01", false, "mhcflurry eval")
	}()
	_ = writer.Close()
	buf, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return calls, string(buf), ret, panicked
}

func writeFakeSSH(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ssh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func errorsAsAttempt(err error, target **sshAttemptError) bool {
	if attempt, ok := err.(*sshAttemptError); ok {
		*target = attempt
		return true
	}
	return false
}
