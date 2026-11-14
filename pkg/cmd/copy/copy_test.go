package copy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRsyncArgs(t *testing.T) {
	t.Run("upload file", func(t *testing.T) {
		args := buildRsyncArgs("ws", "/tmp/local.txt", "/remote/path", true, false)
		assert.Equal(t, []string{"-z", "-e", "ssh", "/tmp/local.txt", "ws:/remote/path"}, args)
	})

	t.Run("upload directory copies contents", func(t *testing.T) {
		args := buildRsyncArgs("ws", "/tmp/mydir", "/remote/path", true, true)
		assert.Equal(t, []string{"-z", "-e", "ssh", "-r", "/tmp/mydir/", "ws:/remote/path"}, args)
	})

	t.Run("upload directory with trailing slash is not doubled", func(t *testing.T) {
		args := buildRsyncArgs("ws", "/tmp/mydir/", "/remote/path", true, true)
		assert.Equal(t, []string{"-z", "-e", "ssh", "-r", "/tmp/mydir/", "ws:/remote/path"}, args)
	})

	t.Run("download file", func(t *testing.T) {
		args := buildRsyncArgs("ws", "/tmp/local.txt", "/remote/path", false, false)
		assert.Equal(t, []string{"-z", "-e", "ssh", "-r", "ws:/remote/path", "/tmp/local.txt"}, args)
	})

	t.Run("download directory copies contents", func(t *testing.T) {
		args := buildRsyncArgs("ws", "/tmp/local", "/remote/path", false, true)
		assert.Equal(t, []string{"-z", "-e", "ssh", "-r", "ws:/remote/path/", "/tmp/local"}, args)
	})
}

func TestBuildSCPArgs(t *testing.T) {
	t.Run("upload file", func(t *testing.T) {
		args := buildSCPArgs("ws", "/tmp/local.txt", "/remote/path", true, false)
		assert.Equal(t, []string{"/tmp/local.txt", "ws:/remote/path"}, args)
	})

	t.Run("upload directory copies contents", func(t *testing.T) {
		args := buildSCPArgs("ws", "/tmp/mydir", "/remote/path", true, true)
		assert.Equal(t, []string{"-r", "/tmp/mydir/.", "ws:/remote/path"}, args)
	})

	t.Run("upload directory with trailing slash is not doubled", func(t *testing.T) {
		args := buildSCPArgs("ws", "/tmp/mydir/", "/remote/path", true, true)
		assert.Equal(t, []string{"-r", "/tmp/mydir/.", "ws:/remote/path"}, args)
	})

	t.Run("download file", func(t *testing.T) {
		args := buildSCPArgs("ws", "/tmp/local.txt", "/remote/path", false, false)
		assert.Equal(t, []string{"-r", "ws:/remote/path", "/tmp/local.txt"}, args)
	})

	t.Run("download directory copies contents", func(t *testing.T) {
		args := buildSCPArgs("ws", "/tmp/local", "/remote/path", false, true)
		assert.Equal(t, []string{"-r", "ws:/remote/path/.", "/tmp/local"}, args)
	})
}

func TestParseCopyArguments_Upload(t *testing.T) {
	ws, remotePath, localPath, isUpload, err := parseCopyArguments("./local.txt", "my-node:/tmp/dest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != "my-node" {
		t.Errorf("expected workspace my-node, got %s", ws)
	}
	if remotePath != "/tmp/dest" {
		t.Errorf("expected remotePath /tmp/dest, got %s", remotePath)
	}
	if localPath != "./local.txt" {
		t.Errorf("expected localPath ./local.txt, got %s", localPath)
	}
	if !isUpload {
		t.Error("expected isUpload=true")
	}
}

func TestParseCopyArguments_Download(t *testing.T) {
	ws, remotePath, localPath, isUpload, err := parseCopyArguments("my-node:/tmp/file", "./local.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != "my-node" {
		t.Errorf("expected workspace my-node, got %s", ws)
	}
	if remotePath != "/tmp/file" {
		t.Errorf("expected remotePath /tmp/file, got %s", remotePath)
	}
	if localPath != "./local.txt" {
		t.Errorf("expected localPath ./local.txt, got %s", localPath)
	}
	if isUpload {
		t.Error("expected isUpload=false")
	}
}

func TestParseCopyArguments_BothLocal(t *testing.T) {
	_, _, _, _, err := parseCopyArguments("./a", "./b")
	if err == nil {
		t.Fatal("expected error when both paths are local")
	}
}

func TestParseCopyArguments_BothRemote(t *testing.T) {
	_, _, _, _, err := parseCopyArguments("ws1:/a", "ws2:/b")
	if err == nil {
		t.Fatal("expected error when both paths are remote")
	}
}

func TestParseWorkspacePath_Local(t *testing.T) {
	ws, fp, err := parseWorkspacePath("/tmp/local/file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != "" {
		t.Errorf("expected empty workspace, got %s", ws)
	}
	if fp != "/tmp/local/file" {
		t.Errorf("expected /tmp/local/file, got %s", fp)
	}
}

func TestParseWorkspacePath_Remote(t *testing.T) {
	ws, fp, err := parseWorkspacePath("my-instance:/remote/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != "my-instance" {
		t.Errorf("expected my-instance, got %s", ws)
	}
	if fp != "/remote/path" {
		t.Errorf("expected /remote/path, got %s", fp)
	}
}

func TestParseWorkspacePath_InvalidMultipleColons(t *testing.T) {
	_, _, err := parseWorkspacePath("ws:path:extra")
	if err == nil {
		t.Fatal("expected error for multiple colons")
	}
}

func TestRemotePathIsDir(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		var gotArgs []string
		runner := func(name string, args ...string) ([]byte, error) {
			gotArgs = append([]string{name}, args...)
			return nil, nil
		}
		assert.True(t, remotePathIsDir("ws", "/remote/path", runner))
		assert.Equal(t, []string{"ssh", "ws", "test", "-d", "'/remote/path'"}, gotArgs)
	})

	t.Run("not a directory", func(t *testing.T) {
		runner := func(name string, args ...string) ([]byte, error) {
			return nil, errors.New("exit status 1")
		}
		assert.False(t, remotePathIsDir("ws", "/remote/file.txt", runner))
	})

	t.Run("quotes are escaped", func(t *testing.T) {
		var gotArgs []string
		runner := func(name string, args ...string) ([]byte, error) {
			gotArgs = append([]string{name}, args...)
			return nil, nil
		}
		remotePathIsDir("ws", "/it's/a/path", runner)
		assert.Equal(t, `'/it'\''s/a/path'`, gotArgs[4])
	})
}

func TestTransferWithFallbackDownloadNormalization(t *testing.T) {
	rsyncAvailable := func() bool { return true }

	t.Run("download probes remote path type and copies directory contents", func(t *testing.T) {
		commands := [][]string{}
		runner := func(name string, args ...string) ([]byte, error) {
			commands = append(commands, append([]string{name}, args...))
			return nil, nil
		}

		err := transferWithFallback("ws", "/tmp/local", "/remote/path", false, runner, rsyncAvailable, nil)
		assert.NoError(t, err)
		assert.Len(t, commands, 2)
		assert.Equal(t, "ssh", commands[0][0])
		assert.Equal(t, []string{"rsync", "-z", "-e", "ssh", "-r", "ws:/remote/path/", "/tmp/local"}, commands[1])
	})

	t.Run("download of a file is not normalized", func(t *testing.T) {
		commands := [][]string{}
		runner := func(name string, args ...string) ([]byte, error) {
			if name == "ssh" {
				return nil, errors.New("exit status 1")
			}
			commands = append(commands, append([]string{name}, args...))
			return nil, nil
		}

		err := transferWithFallback("ws", "/tmp/local.txt", "/remote/file.txt", false, runner, rsyncAvailable, nil)
		assert.NoError(t, err)
		assert.Equal(t, [][]string{{"rsync", "-z", "-e", "ssh", "-r", "ws:/remote/file.txt", "/tmp/local.txt"}}, commands)
	})
}

func TestTransferWithFallback(t *testing.T) {
	rsyncAvailable := func() bool { return true }

	t.Run("rsync success", func(t *testing.T) {
		calls := []string{}
		runner := func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name)
			return []byte("ok"), nil
		}

		onFallbackCalled := false
		err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner, rsyncAvailable, func(reason string) {
			onFallbackCalled = true
		})
		assert.NoError(t, err)
		assert.False(t, onFallbackCalled)
		assert.Equal(t, []string{"rsync"}, calls)
	})

	t.Run("rsync not installed locally skips straight to scp", func(t *testing.T) {
		calls := []string{}
		runner := func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name)
			return []byte("ok"), nil
		}

		reasons := []string{}
		err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner, func() bool { return false }, func(reason string) {
			reasons = append(reasons, reason)
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"scp"}, calls)
		assert.Len(t, reasons, 1)
		assert.Contains(t, reasons[0], "Install rsync for faster transfers")
	})

	t.Run("rsync missing on instance falls back with install hint", func(t *testing.T) {
		calls := []string{}
		runner := func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name)
			if name == "rsync" {
				return []byte("bash: rsync: command not found"), errors.New("exit status 127")
			}
			return []byte("scp ok"), nil
		}

		reasons := []string{}
		err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner, rsyncAvailable, func(reason string) {
			reasons = append(reasons, reason)
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"rsync", "scp"}, calls)
		assert.Len(t, reasons, 1)
		assert.Contains(t, reasons[0], "Install rsync on the instance for faster transfers")
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

		err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner, rsyncAvailable, func(reason string) {
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

		err := transferWithFallback("ws", "/tmp/local.txt", "/remote/path", true, runner, rsyncAvailable, func(reason string) {})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rsync failed: exit status 1")
		assert.Contains(t, err.Error(), "scp fallback failed")
		assert.Contains(t, err.Error(), "rsync output")
		assert.Contains(t, err.Error(), "scp output")
		assert.NotContains(t, err.Error(), "rsync failed: rsync failed:")
	})
}
