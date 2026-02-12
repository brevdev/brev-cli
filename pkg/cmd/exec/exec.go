package exec

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/writeconnectionevent"
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
	util.WorkspaceStartStore
	refresh.RefreshStore
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
}

func NewCmdExec(t *terminal.Terminal, store ExecStore, noLoginStartStore ExecStore) *cobra.Command {
	var host bool
	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "exec [instance...] <command>",
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

const pollTimeout = 10 * time.Minute

func runExecCommand(t *terminal.Terminal, sstore ExecStore, workspaceNameOrID string, host bool, command string) error {
	s := t.NewSpinner()
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(sstore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if workspace.Status == "STOPPED" { // we start the env for the user
		err = util.StartWorkspaceIfStopped(t, s, sstore, workspaceNameOrID, workspace, pollTimeout)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	err = util.PollUntil(s, workspace.ID, "RUNNING", sstore, " waiting for instance to be ready...", pollTimeout)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	refreshRes := refresh.RunRefreshAsync(sstore)

	workspace, err = util.GetUserWorkspaceByNameOrIDErr(sstore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspace.Status != "RUNNING" {
		return breverrors.New("Instance is not running")
	}

	localIdentifier := workspace.GetLocalIdentifier()
	if host {
		localIdentifier = workspace.GetHostIdentifier()
	}

	sshName := string(localIdentifier)

	err = refreshRes.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = util.WaitForSSHToBeAvailable(sshName, s)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	// we don't care about the error here but should log with sentry
	// legacy environments wont support this and cause errrors,
	// but we don't want to block the user from using the shell
	_ = writeconnectionevent.WriteWCEOnEnv(sstore, workspace.DNS)
	err = runSSH(sshName, command)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	// Call analytics for exec
	userID := ""
	user, err := sstore.GetCurrentUser()
	if err != nil {
		userID = workspace.CreatedByUserID
	} else {
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

	return nil
}

func runSSH(sshAlias string, command string) error {
	sshAgentEval := "eval $(ssh-agent -s)"

	// Non-interactive: run command and pipe stdout/stderr
	// Escape the command for passing to SSH
	escapedCmd := strings.ReplaceAll(command, "'", "'\\''")
	cmd := fmt.Sprintf("ssh %s '%s'", sshAlias, escapedCmd)

	cmd = fmt.Sprintf("%s && %s", sshAgentEval, cmd)
	sshCmd := exec.Command("bash", "-c", cmd) //nolint:gosec //cmd is user input
	sshCmd.Stderr = os.Stderr
	sshCmd.Stdout = os.Stdout
	// Don't attach stdin - exec is non-interactive

	err := sshCmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
