package spark

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type capturingExecutor struct {
	argv []string
	err  error
}

func (c *capturingExecutor) Run(argv []string) error {
	c.argv = argv
	return c.err
}

func TestBuildSSHArgsOrdersOptions(t *testing.T) {
	host := Host{
		Alias:        "spark-one",
		Hostname:     "h1",
		User:         "u1",
		Port:         2222,
		IdentityFile: "/id",
		Options: map[string]string{
			"ProxyJump":             "jump",
			"StrictHostKeyChecking": "no",
		},
	}

	args := BuildSSHArgs(host)
	require.Equal(t, []string{
		"ssh",
		"-i", "/id",
		"-p", "2222",
		"-o", "ProxyJump=jump",
		"-o", "StrictHostKeyChecking=no",
		"u1@h1",
	}, args)
}

func TestSSHRunnerRequiresIdentityFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	exec := &capturingExecutor{}

	runner := NewSSHRunner(fs, exec)
	err := runner.Run(Host{Alias: "spark-one"})
	require.Error(t, err)
}

func TestSSHRunnerRunsCommand(t *testing.T) {
	fs := afero.NewMemMapFs()
	exec := &capturingExecutor{}
	err := afero.WriteFile(fs, "/keys/id", []byte("pem"), 0o600)
	require.NoError(t, err)

	host := Host{
		Alias:        "spark-one",
		Hostname:     "h1",
		User:         "u1",
		Port:         22,
		IdentityFile: "/keys/id",
		Options: map[string]string{
			"ProxyJump": "jump",
		},
	}

	runner := NewSSHRunner(fs, exec)
	err = runner.Run(host)
	require.NoError(t, err)

	expected := []string{"ssh", "-i", "/keys/id", "-p", "22", "-o", "ProxyJump=jump", "u1@h1"}
	require.Equal(t, expected, exec.argv)
}
