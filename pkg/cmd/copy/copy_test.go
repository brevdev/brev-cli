package copy

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRsyncArgs(t *testing.T) {
	t.Run("upload file", func(t *testing.T) {
		args := buildRsyncArgs("ws", "/tmp/local.txt", "/remote/path", true)
		assert.Equal(t, []string{"-z", "-e", "ssh", "/tmp/local.txt", "ws:/remote/path"}, args)
	})

	t.Run("upload directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		localDir := filepath.Join(tmpDir, "mydir")
		err := os.MkdirAll(localDir, 0o755)
		assert.NoError(t, err)

		args := buildRsyncArgs("ws", localDir, "/remote/path", true)
		assert.Equal(t, []string{"-z", "-e", "ssh", "-r", localDir, "ws:/remote/path"}, args)
	})

	t.Run("download path", func(t *testing.T) {
		args := buildRsyncArgs("ws", "/tmp/local.txt", "/remote/path", false)
		assert.Equal(t, []string{"-z", "-e", "ssh", "-r", "ws:/remote/path", "/tmp/local.txt"}, args)
	})
}

func TestBuildSCPArgs(t *testing.T) {
	t.Run("upload file", func(t *testing.T) {
		args := buildSCPArgs("ws", "/tmp/local.txt", "/remote/path", true)
		assert.Equal(t, []string{"/tmp/local.txt", "ws:/remote/path"}, args)
	})

	t.Run("upload directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		localDir := filepath.Join(tmpDir, "mydir")
		err := os.MkdirAll(localDir, 0o755)
		assert.NoError(t, err)

		args := buildSCPArgs("ws", localDir, "/remote/path", true)
		assert.Equal(t, []string{"-r", localDir, "ws:/remote/path"}, args)
	})

	t.Run("download path", func(t *testing.T) {
		args := buildSCPArgs("ws", "/tmp/local.txt", "/remote/path", false)
		assert.Equal(t, []string{"-r", "ws:/remote/path", "/tmp/local.txt"}, args)
	})
}

func TestTransferWithFallback(t *testing.T) {
	t.Run("rsync success", func(t *testing.T) {
		calls := []string{}
		runner := func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name)
			return []byte("ok"), nil
		}

		onFallbackCalled := false
		err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner, func() {
			onFallbackCalled = true
		})
		assert.NoError(t, err)
		assert.False(t, onFallbackCalled)
		assert.Equal(t, []string{"rsync"}, calls)
	})

	t.Run("rsync fails and scp succeeds", func(t *testing.T) {
		calls := []string{}
		runner := func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name)
			if name == "rsync" {
				return []byte("rsync failed"), errors.New("exit status 1")
			}
			return []byte("scp ok"), nil
		}

		err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner, func() {
			calls = append(calls, "fallback")
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"rsync", "fallback", "scp"}, calls)
	})

	t.Run("rsync fails and scp fails", func(t *testing.T) {
		runner := func(name string, args ...string) ([]byte, error) {
			if name == "rsync" {
				return []byte("rsync output"), errors.New("exit status 1")
			}
			return []byte("scp output"), errors.New("exit status 1")
		}

		err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner, func() {})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rsync failed: exit status 1")
		assert.Contains(t, err.Error(), "scp fallback failed")
		assert.Contains(t, err.Error(), "rsync output")
		assert.Contains(t, err.Error(), "scp output")
		assert.NotContains(t, err.Error(), "rsync failed: rsync failed:")
	})
}
