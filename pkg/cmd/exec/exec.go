package exec

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/briandowns/spinner"

	"github.com/spf13/cobra"
)

var (
	execLong    = "Execute a script or command on a workspace without manually setting up SSH"
	execExample = `  brev exec workspace-name --script ./setup.sh
  brev exec workspace-name --command "ls -la"
  brev exec workspace-name --script ./deploy.sh --async`
)

type ExecStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	refresh.RefreshStore
	completions.CompletionStore
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdExec(t *terminal.Terminal, store ExecStore, noLoginStore ExecStore) *cobra.Command {
	var scriptPath string
	var command string
	var workingDir string
	var async bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "exec",
		DisableFlagsInUseLine: true,
		Short:                 "Execute a script or command on a workspace",
		Long:                  execLong,
		Example:               execExample,
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceNameOrID := args[0]

			if scriptPath == "" && command == "" {
				return breverrors.NewValidationError("either --script or --command must be provided")
			}
			if scriptPath != "" && command != "" {
				return breverrors.NewValidationError("cannot specify both --script and --command")
			}

			return runExec(t, store, workspaceNameOrID, scriptPath, command, workingDir, async)
		},
	}

	cmd.Flags().StringVarP(&scriptPath, "script", "s", "", "Path to script file to execute")
	cmd.Flags().StringVarP(&command, "command", "c", "", "Command to execute")
	cmd.Flags().StringVarP(&workingDir, "dir", "d", "", "Working directory (default: workspace directory)")
	cmd.Flags().BoolVarP(&async, "async", "a", false, "Run command asynchronously (fire and forget)")

	return cmd
}

func runExec(t *terminal.Terminal, store ExecStore, workspaceNameOrID, scriptPath, command, workingDir string, async bool) error {
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(store, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if workspace.Status != entity.Running {
		return breverrors.NewValidationError(fmt.Sprintf("workspace '%s' is not in running state (current state: %s)", workspace.Name, workspace.Status))
	}

	refreshRes := refresh.RunRefreshAsync(store)
	err = refreshRes.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	localIdentifier := workspace.GetLocalIdentifier()
	sshAlias := string(localIdentifier)

	s := t.NewSpinner()
	err = waitForSSH(sshAlias, s)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if scriptPath != "" {
		// #nosec G304 -- scriptPath is provided by user via CLI flag, intentional file read
		scriptContent, err := os.ReadFile(scriptPath)
		if err != nil {
			return breverrors.WrapAndTrace(fmt.Errorf("failed to read script: %w", err))
		}
		return executeScriptViaSSH(t, sshAlias, string(scriptContent), workingDir, async)
	}

	return executeCommandViaSSH(t, sshAlias, command, workingDir, async)
}

func executeScriptViaSSH(t *terminal.Terminal, sshAlias, scriptContent, workingDir string, async bool) error {
	tmpFile, err := os.CreateTemp("", "brev-exec-*.sh")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			t.Vprintf(t.Yellow("Warning: failed to remove temp file: %v\n", removeErr))
		}
	}()

	_, err = tmpFile.WriteString(scriptContent)
	if err != nil {
		_ = tmpFile.Close()
		return breverrors.WrapAndTrace(err)
	}
	if err = tmpFile.Close(); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	remotePath := "/tmp/brev-exec-script.sh"

	t.Vprintf(t.Green("Copying script to workspace...\n"))

	var scpErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			t.Vprintf(t.Yellow("Retrying SCP (attempt %d/3)...\n", attempt))
			time.Sleep(3 * time.Second)
		}

		scpCmd := exec.Command("scp",
			"-o", "ConnectTimeout=40",
			"-o", "ServerAliveInterval=20",
			"-o", "ServerAliveCountMax=5",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			tmpFile.Name(),
			fmt.Sprintf("%s:%s", sshAlias, remotePath))
		scpCmd.Env = os.Environ()

		output, err := scpCmd.CombinedOutput()
		if err == nil {
			scpErr = nil
			break
		}

		scpErr = fmt.Errorf("scp failed (attempt %d): %s\nOutput: %s", attempt, err.Error(), string(output))
	}

	if scpErr != nil {
		return breverrors.WrapAndTrace(scpErr)
	}

	execCmd := fmt.Sprintf("chmod +x %s && bash %s && rm -f %s", remotePath, remotePath, remotePath)
	if workingDir != "" {
		execCmd = fmt.Sprintf("cd %s && %s", workingDir, execCmd)
	}

	return executeCommandViaSSH(t, sshAlias, execCmd, "", async)
}

func executeCommandViaSSH(t *terminal.Terminal, sshAlias, command, workingDir string, async bool) error {
	execCmd := command
	if workingDir != "" {
		execCmd = fmt.Sprintf("cd %s && %s", workingDir, execCmd)
	}

	escapedCmd := strings.ReplaceAll(execCmd, "'", "'\"'\"'")

	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			t.Vprintf(t.Yellow("Retrying SSH command (attempt %d/%d)...\n", attempt, maxRetries))
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}

		// #nosec G204 -- sshAlias and escapedCmd are controlled inputs from workspace config and user CLI flags
		cmd := exec.Command("ssh",
			"-o", "ConnectTimeout=30",
			"-o", "ServerAliveInterval=10",
			"-o", "ServerAliveCountMax=3",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			sshAlias,
			escapedCmd)
		cmd.Env = os.Environ()

		if async {
			t.Vprintf(t.Green("Running command asynchronously...\n"))
			err := cmd.Start()
			if err != nil {
				lastErr = err
				continue
			}
			t.Vprintf(t.Green("Command started successfully (PID: %d)\n"), cmd.Process.Pid)
			return nil
		}

		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		t.Vprintf(t.Green("Executing command...\n"))
		err := cmd.Run()
		if err == nil {
			return nil
		}

		lastErr = err

		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 255 {
				t.Vprintf(t.Yellow("SSH connection closed, retrying...\n"))
				continue
			}
		}

		return breverrors.WrapAndTrace(fmt.Errorf("command execution failed: %w", err))
	}

	return breverrors.WrapAndTrace(fmt.Errorf("command execution failed after %d attempts: %w", maxRetries, lastErr))
}

func waitForSSH(sshAlias string, s *spinner.Spinner) error {
	const maxRetries = 60
	const retryInterval = 2 * time.Second

	counter := 0
	s.Suffix = " waiting for SSH connection..."
	s.Start()
	defer s.Stop()

	for counter < maxRetries {
		cmd := exec.Command("ssh",
			"-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			sshAlias,
			"echo", "test")
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err == nil {
			time.Sleep(2 * time.Second)
			return nil
		}

		outputStr := string(out)
		lines := strings.Split(outputStr, "\n")

		if len(lines) >= 2 {
			stdErr := lines[1]

			if !store.SatisfactorySSHErrMessage(stdErr) {
				if !strings.Contains(stdErr, "Connection closed") &&
					!strings.Contains(stdErr, "Connection refused") &&
					!strings.Contains(stdErr, "Connection timed out") {
					return breverrors.WrapAndTrace(fmt.Errorf("SSH connection failed: %s", stdErr))
				}
			}
		}

		counter++
		time.Sleep(retryInterval)
	}

	return breverrors.WrapAndTrace(fmt.Errorf("SSH connection timeout after %d attempts", maxRetries))
}
