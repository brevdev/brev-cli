package spark

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"

	"github.com/spf13/afero"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type SSHRunner struct {
	executor Executor
	fs       afero.Fs
}

func NewDefaultSSHRunner(fs afero.Fs) SSHRunner {
	return NewSSHRunner(fs, ProcessExecutor{})
}

func NewSSHRunner(fs afero.Fs, executor Executor) SSHRunner {
	return SSHRunner{
		executor: executor,
		fs:       fs,
	}
}

func (r SSHRunner) Run(host Host) error {
	if host.IdentityFile == "" {
		return breverrors.WrapAndTrace(fmt.Errorf("missing identity file for %s", host.Alias))
	}

	exists, err := afero.Exists(r.fs, host.IdentityFile)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !exists {
		return breverrors.WrapAndTrace(fmt.Errorf("identity file not found at %s", host.IdentityFile))
	}

	argv := BuildSSHArgs(host)
	return breverrors.WrapAndTrace(r.executor.Run(argv))
}

func BuildSSHArgs(host Host) []string {
	args := []string{
		"ssh",
		"-i", host.IdentityFile,
		"-p", strconv.Itoa(host.Port),
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

	target := host.Hostname
	if host.Alias != "" {
		target = host.Alias
	}
	args = append(args, fmt.Sprintf("%s@%s", host.User, target))

	return args
}

type ProcessExecutor struct{}

func (ProcessExecutor) Run(argv []string) error {
	if len(argv) == 0 {
		return breverrors.WrapAndTrace(fmt.Errorf("no command provided"))
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
