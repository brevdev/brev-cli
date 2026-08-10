package sudo

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/require"
)

type sudoTestConfirmer struct{}

func (sudoTestConfirmer) ConfirmYesNo(string) bool { return true }

func TestSystemGater_UncachedNonInteractiveSudoFailureIsReturned(t *testing.T) {
	stdin, err := os.CreateTemp(t.TempDir(), "stdin")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, stdin.Close()) })

	probeErr := errors.New("sudo credentials unavailable")
	runCalls := 0
	gater := &systemGater{
		checkStatus: func() Status { return StatusUncached },
		runCommand: func(cmd *exec.Cmd) error {
			runCalls++
			require.Equal(t, []string{"sudo", "-n", "-v"}, cmd.Args)
			return probeErr
		},
		stdin: stdin,
	}

	err = gater.Gate(terminal.New(), sudoTestConfirmer{}, "Node-wide Brev SSH cleanup", true)
	require.ErrorIs(t, err, probeErr)
	require.ErrorContains(t, err, "sudo authentication unavailable without an interactive terminal")
	require.Equal(t, 1, runCalls)
}
