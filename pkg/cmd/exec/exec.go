package exec

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/hashicorp/go-multierror"

	"github.com/spf13/cobra"
)

var (
	execLong    = "Execute a command on one or more instances non-interactively"
	execExample = `  # Run a command on an instance
  brev exec my-instance "nvidia-smi"
  brev exec my-instance "python train.py"

  # Run a command on multiple instances
  brev exec instance1 instance2 instance3 "nvidia-smi"

  # Run a script file on the instance (@ prefix reads local file)
  brev exec my-instance @setup.sh
  brev exec my-instance @scripts/deploy.sh

  # Run a background job
  brev exec my-instance "nohup python train.py > /dev/null 2>&1 &"

  # Chain: create and run a command (reads instance names from stdin)
  brev create my-instance | brev exec "nvidia-smi"

  # Run command on a cluster
  brev create my-cluster --count 3 | brev exec "nvidia-smi"

  # Pipeline: create, setup, then run
  brev create my-gpu | brev exec "pip install torch" | brev exec "python train.py"

  # SSH into the host machine instead of the container
  brev exec my-instance --host "nvidia-smi"`
)

type ExecStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	refresh.RefreshStore
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
}

func NewCmdExec(t *terminal.Terminal, store ExecStore, noLoginStartStore ExecStore) *cobra.Command {
	var host bool
	cmd := &cobra.Command{
		Annotations: map[string]string{"access": ""},
		Use: `exec [instance...] "<command>"
  brev exec [instance...] @<script-file-present-on-local>`,
		DisableFlagsInUseLine: true,
		Short:                 "Execute a command on instance(s)",
		Long:                  execLong,
		Example:               execExample,
		Args:                  cobra.MinimumNArgs(1),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Last argument is the command, rest are instance names
			command := args[len(args)-1]
			instanceArgs := args[:len(args)-1]

			// Get instance names from args or stdin
			instanceNames, err := getInstanceNames(instanceArgs)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			// Parse command (can be inline or @filepath)
			cmdToRun, err := parseCommand(command)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			if cmdToRun == "" {
				return breverrors.NewValidationError("command is required")
			}

			// Run on each instance
			var errors error
			for _, instanceName := range instanceNames {
				if len(instanceNames) > 1 {
					fmt.Fprintf(os.Stderr, "\n=== %s ===\n", instanceName)
				}
				err = runExecCommand(t, store, instanceName, host, cmdToRun)
				if err != nil {
					if len(instanceNames) > 1 {
						fmt.Fprintf(os.Stderr, "Error on %s: %v\n", instanceName, err)
						errors = multierror.Append(errors, err)
						continue
					}
					return breverrors.WrapAndTrace(err)
				}
				// Output instance name for chaining (only if stdout is piped)
				if isPiped() {
					fmt.Println(instanceName)
				}
			}
			if errors != nil {
				return breverrors.WrapAndTrace(errors)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&host, "host", "", false, "ssh into the host machine instead of the container")

	return cmd
}

// isPiped returns true if stdout is piped to another command
func isPiped() bool {
	stat, _ := os.Stdout.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// getInstanceNames gets instance names from args or stdin (supports multiple)
func getInstanceNames(args []string) ([]string, error) {
	var names []string

	// Add names from args
	names = append(names, args...)

	// Check if stdin is piped
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Stdin is piped, read instance names (one per line)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			name := strings.TrimSpace(scanner.Text())
			if name != "" {
				names = append(names, name)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}

	if len(names) == 0 {
		return nil, breverrors.NewValidationError("instance name required: provide as argument or pipe from another command")
	}

	return names, nil
}

// parseCommand parses the command string, loading from file if prefixed with @
func parseCommand(command string) (string, error) {
	if command == "" {
		return "", nil
	}

	// If prefixed with @, read from file
	if strings.HasPrefix(command, "@") {
		filePath := strings.TrimPrefix(command, "@")
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		return string(content), nil
	}

	return command, nil
}

func runExecCommand(t *terminal.Terminal, sstore ExecStore, workspaceNameOrID string, host bool, command string) error {
	// Determine SSH alias: use the workspace name directly (with -host suffix if needed)
	sshName := workspaceNameOrID
	if host {
		sshName = workspaceNameOrID + "-host"
	}

	// Fire SSH immediately with a short timeout — skip all status checks for speed.
	// SSH multiplexing (ControlMaster) in the config means subsequent
	// calls reuse an existing connection and are near-instant.
	// Use a 5-second connect timeout so we fail fast if the instance is down.
	err := runExecSSH(sshName, command, 5)
	if err == nil {
		// Success — fire analytics in background and return
		go trackExecAnalytics(sstore, workspaceNameOrID)
		return nil
	}

	// The first attempt already is the execution. A non-zero remote status, or
	// an SSH session that died after the command may have started, must not be
	// run again: the instance can still be RUNNING with the original process
	// (or its output) in place. Only a failure from before the session exists
	// may fall through to the refresh-and-retry path below.
	if !sshFailureAllowsReplay(err) {
		return breverrors.WrapAndTrace(err)
	}

	// SSH failed — now check what's going on with the instance
	fmt.Fprintf(os.Stderr, "Connection failed, checking instance status...\n")

	workspace, lookupErr := util.GetUserWorkspaceByNameOrIDErr(sstore, workspaceNameOrID)
	if lookupErr != nil {
		return breverrors.NewValidationError(fmt.Sprintf(
			"could not look up instance %q: %s\ncheck your instances with: brev ls",
			workspaceNameOrID, breverrors.Root(lookupErr)))
	}

	if err := util.RequireRunning(workspace); err != nil {
		return err //nolint:wrapcheck // do not present stack trace for this error
	}

	// Instance is RUNNING but SSH failed — maybe still booting, refresh and retry
	// a few times.
	s := t.NewSpinner()
	refreshRes := refresh.RunRefreshAsync(sstore)
	if err = refreshRes.Await(); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	localIdentifier := workspace.GetLocalIdentifier()
	if host {
		localIdentifier = workspace.GetHostIdentifier()
	}
	sshName = string(localIdentifier)

	err = util.WaitForSSHToBeAvailable(sshName, s, func() error { return refresh.RunRefreshAsync(sstore).Await() })
	if err != nil {
		// Wrapping here would sever the error type the renderer uses to decide
		// whether to dump a stack trace, and the message already names the
		// instance and what to check.
		return err //nolint:wrapcheck // do not present stack trace for this error
	}
	err = runExecSSH(sshName, command, 10)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	go trackExecAnalytics(sstore, workspaceNameOrID)
	return nil
}

func trackExecAnalytics(sstore ExecStore, workspaceNameOrID string) {
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(sstore, workspaceNameOrID)
	if err != nil {
		return
	}
	userID := workspace.CreatedByUserID
	user, err := sstore.GetCurrentUser()
	if err == nil {
		userID = user.ID
	}
	data := analytics.EventData{
		EventName: "Brev Exec",
		UserID:    userID,
		Properties: map[string]string{
			"instanceId": workspace.ID,
		},
	}
	_ = analytics.TrackEvent(data)
}

// runExecSSH is the SSH invocation used by brev exec. Tests replace it.
var runExecSSH = runSSHWithTimeout

// sshAttemptError is a failed SSH invocation of the remote command.
// exitCode 255 is ssh's own failure; any other code is the remote command's.
// diag is ssh's private log (ssh -E), not the remote command's stderr.
type sshAttemptError struct {
	err      error
	exitCode int
	diag     string
}

func (e *sshAttemptError) Error() string {
	if e == nil || e.err == nil {
		return "ssh failed"
	}
	return e.err.Error()
}

func (e *sshAttemptError) Unwrap() error { return e.err }

func newSSHAttemptError(err error, diag string) error {
	code := -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	}
	return &sshAttemptError{err: err, exitCode: code, diag: diag}
}

// sshFailureAllowsReplay reports whether the remote command is known not to
// have started, so the caller may wait for SSH and run it once. A non-zero
// remote status, a session that dropped, or an unrecognized ssh failure is
// not replayed: the command may already be running.
func sshFailureAllowsReplay(err error) bool {
	var attempt *sshAttemptError
	if !errors.As(err, &attempt) {
		return false
	}
	// OpenSSH uses 255 for its own failures and passes every other status
	// through from the remote command.
	if attempt.exitCode != 255 {
		return false
	}
	return sshDiagIsPreSession(attempt.diag)
}

// sshDiagIsPreSession reports whether ssh failed before a session existed.
// Only the last non-empty line of the -E log counts. An earlier
// "ssh: connect to host" line is normal when another address then opens a
// session that later drops, and that command must not be run again.
// The line has to start with an OpenSSH pre-session prefix. A bare errno
// string is not enough: "Read from remote host X: Connection timed out" is
// written after the session is up. The strerror after the prefix is not
// matched, so a translated message still counts.
func sshDiagIsPreSession(diag string) bool {
	line := lastNonEmptyDiagLine(diag)
	prefixes := []string{
		"ssh: connect to host ",
		"ssh: Could not resolve hostname",
		"kex_exchange_identification:",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func lastNonEmptyDiagLine(diag string) string {
	var line string
	for _, raw := range strings.Split(diag, "\n") {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			line = trimmed
		}
	}
	return line
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func runSSHWithTimeout(sshAlias string, command string, connectTimeoutSecs int) error {
	// Non-interactive: run command and pipe stdout/stderr
	// Escape the command for passing to SSH
	escapedCmd := strings.ReplaceAll(command, "'", "'\\''")
	// -T disables pseudo-terminal allocation (no "Pseudo-terminal will not be allocated" warning)
	// Only start ssh-agent if one isn't already running (avoids orphaned agent processes)
	// -E keeps ssh's own errors out of the remote command's stderr so a dropped
	// session is not mistaken for "never connected".
	agentCmd := `if [ -z "$SSH_AUTH_SOCK" ]; then eval $(ssh-agent -s) > /dev/null; fi`
	diagFile, diagErr := os.CreateTemp("", "brev-exec-ssh-")
	if diagErr != nil {
		return breverrors.WrapAndTrace(diagErr)
	}
	diagPath := diagFile.Name()
	_ = diagFile.Close()
	defer os.Remove(diagPath)

	cmd := fmt.Sprintf("%s && ssh -T -E %s -o ConnectTimeout=%d -o LogLevel=ERROR %s '%s'",
		agentCmd, shellSingleQuote(diagPath), connectTimeoutSecs, sshAlias, escapedCmd)

	sshCmd := exec.Command("bash", "-c", cmd) //nolint:gosec // cmd is user input
	sshCmd.Stderr = os.Stderr
	sshCmd.Stdout = os.Stdout
	// Don't attach stdin - exec is non-interactive

	err := sshCmd.Run()
	diagBytes, readErr := os.ReadFile(diagPath)
	diag := ""
	if readErr == nil {
		diag = string(diagBytes)
	}
	if err != nil {
		if diag != "" {
			_, _ = os.Stderr.WriteString(diag)
			if !strings.HasSuffix(diag, "\n") {
				_, _ = fmt.Fprintln(os.Stderr)
			}
		}
		return newSSHAttemptError(err, diag)
	}
	return nil
}
