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

		fellBack, err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner)
		assert.NoError(t, err)
		assert.False(t, fellBack)
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

		fellBack, err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner)
		assert.NoError(t, err)
		assert.True(t, fellBack)
		assert.Equal(t, []string{"rsync", "scp"}, calls)
	})

	t.Run("rsync fails and scp fails", func(t *testing.T) {
		runner := func(name string, args ...string) ([]byte, error) {
			if name == "rsync" {
				return []byte("rsync output"), errors.New("exit status 1")
			}
			return []byte("scp output"), errors.New("exit status 1")
		}

		fellBack, err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner)
		assert.Error(t, err)
		assert.True(t, fellBack)
		assert.Contains(t, err.Error(), "rsync failed")
		assert.Contains(t, err.Error(), "scp fallback failed")
		assert.Contains(t, err.Error(), "rsync output")
		assert.Contains(t, err.Error(), "scp output")
	})
}
