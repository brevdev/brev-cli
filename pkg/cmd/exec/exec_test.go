package exec

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type noRetryExecStore struct {
	ExecStore
	lookupCalls int
}

func (s *noRetryExecStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return nil, nil
}

func (s *noRetryExecStore) GetCurrentUser() (*entity.User, error) {
	s.lookupCalls++
	return nil, errors.New("workspace lookup should not run for a remote command failure")
}

func TestRunExecCommandDoesNotRetryRemoteCommandFailure(t *testing.T) {
	fakeBin := t.TempDir()
	fakeSSH := filepath.Join(fakeBin, "ssh")
	if err := os.WriteFile(fakeSSH, []byte("#!/bin/sh\nprintf '[]\\n'\nprintf 'Error: No such object: nosuchjob\\n' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SSH_AUTH_SOCK", "test-agent")

	store := &noRetryExecStore{}
	err := runExecCommand(terminal.New(), store, "test-instance", false, "docker inspect nosuchjob")
	if err == nil {
		t.Fatal("expected the remote command failure to be returned")
	}
	if store.lookupCalls != 0 {
		t.Fatalf("remote command failure entered the connection-recovery path %d time(s); want 0", store.lookupCalls)
	}
}
