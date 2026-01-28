package shell

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/writeconnectionevent"
	"github.com/briandowns/spinner"

	"github.com/spf13/cobra"
)

var (
	openLong    = "[command in beta] This will shell in to your instance"
	openExample = `  # SSH into an instance by name or ID
  brev shell instance_id_or_name
  brev shell my-instance

  # Run a command on the instance (non-interactive, pipes stdout/stderr)
  brev shell my-instance -c "nvidia-smi"
  brev shell my-instance -c "python train.py"

  # Run a script file on the instance
  brev shell my-instance -c @setup.sh

  # Chain: provision and run a command (reads instance name from stdin)
  brev provision --name my-instance | brev shell -c "nvidia-smi"

  # Create a GPU instance and SSH into it immediately (use command substitution for interactive shell)
  brev shell $(brev provision --name my-instance)

  # Create with specific GPU and connect
  brev shell $(brev gpus --gpu-name A100 | brev gpu-create --name ml-box)

  # SSH into the host machine instead of the container
  brev shell my-instance --host`
)

type ShellStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	refresh.RefreshStore
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	StartWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentUserKeys() (*entity.UserKeys, error)
}

func NewCmdShell(t *terminal.Terminal, store ShellStore, noLoginStartStore ShellStore) *cobra.Command {
	var host bool
	var command string
	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "shell",
		Aliases:               []string{"ssh"},
		DisableFlagsInUseLine: true,
		Short:                 "[beta] Open a shell in your instance",
		Long:                  openLong,
		Example:               openExample,
		Args:                  cmderrors.TransformToValidationError(cobra.RangeArgs(0, 1)),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get instance name from args or stdin
			instanceName, err := getInstanceName(args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			// Parse command (can be inline or @filepath)
			cmdToRun, err := parseCommand(command)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			err = runShellCommand(t, store, instanceName, host, cmdToRun)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&host, "host", "", false, "ssh into the host machine instead of the container")
	cmd.Flags().StringVarP(&command, "command", "c", "", "command to run on the instance (use @filename to run a script file)")

	return cmd
}

// getInstanceName gets the instance name from args or stdin
func getInstanceName(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	// Check if stdin is piped
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Stdin is piped, read instance name
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			name := strings.TrimSpace(scanner.Text())
			if name != "" {
				return name, nil
			}
		}
	}

	return "", breverrors.NewValidationError("instance name required: provide as argument or pipe from another command")
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

func runShellCommand(t *terminal.Terminal, sstore ShellStore, workspaceNameOrID string, host bool, command string) error {
	s := t.NewSpinner()
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(sstore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if workspace.Status == "STOPPED" { // we start the env for the user
		err = startWorkspaceIfStopped(t, s, sstore, workspaceNameOrID, workspace)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	err = pollUntil(s, workspace.ID, "RUNNING", sstore, " waiting for instance to be ready...")
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
	err = waitForSSHToBeAvailable(sshName, s)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	// we don't care about the error here but should log with sentry
	// legacy environments wont support this and cause errrors,
	// but we don't want to block the user from using the shell
	_ = writeconnectionevent.WriteWCEOnEnv(sstore, workspace.DNS)
	err = runSSH(workspace, sshName, command)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	// Call analytics for shell
	userID := ""
	user, err := sstore.GetCurrentUser()
	if err != nil {
		userID = workspace.CreatedByUserID
	} else {
		userID = user.ID
	}
	data := analytics.EventData{
		EventName: "Brev Open",
		UserID:    userID,
		Properties: map[string]string{
			"instanceId": workspace.ID,
		},
	}
	_ = analytics.TrackEvent(data)

	return nil
}

func waitForSSHToBeAvailable(sshAlias string, s *spinner.Spinner) error {
	counter := 0
	s.Suffix = " waiting for SSH connection to be available"
	s.Start()
	for {
		cmd := exec.Command("ssh", "-o", "ConnectTimeout=10", sshAlias, "echo", " ")
		out, err := cmd.CombinedOutput()
		if err == nil {
			s.Stop()
			return nil
		}

		outputStr := string(out)
		stdErr := strings.Split(outputStr, "\n")[1]

		if counter == 40 || !store.SatisfactorySSHErrMessage(stdErr) {
			return breverrors.WrapAndTrace(errors.New("\n" + stdErr))
		}

		counter++
		time.Sleep(1 * time.Second)
	}
}

func runSSH(_ *entity.Workspace, sshAlias string, command string) error {
	sshAgentEval := "eval $(ssh-agent -s)"

	var cmd string
	if command != "" {
		// Non-interactive: run command and pipe stdout/stderr
		// Escape the command for passing to SSH
		escapedCmd := strings.ReplaceAll(command, "'", "'\\''")
		cmd = fmt.Sprintf("ssh %s '%s'", sshAlias, escapedCmd)
	} else {
		// Interactive shell
		cmd = fmt.Sprintf("ssh %s", sshAlias)
	}

	cmd = fmt.Sprintf("%s && %s", sshAgentEval, cmd)
	sshCmd := exec.Command("bash", "-c", cmd) //nolint:gosec //cmd is user input
	sshCmd.Stderr = os.Stderr
	sshCmd.Stdout = os.Stdout

	// Only attach stdin for interactive sessions
	if command == "" {
		sshCmd.Stdin = os.Stdin
	}

	err := hello.SetHasRunShell(true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = sshCmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func startWorkspaceIfStopped(t *terminal.Terminal, s *spinner.Spinner, tstore ShellStore, wsIDOrName string, workspace *entity.Workspace) error {
	activeOrg, err := tstore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaces, err := tstore.GetWorkspaceByNameOrID(activeOrg.ID, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	startedWorkspace, err := tstore.StartWorkspace(workspaces[0].ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprintf("%s", t.Yellow("Instance %s is starting. \n\n", startedWorkspace.Name))
	err = pollUntil(s, workspace.ID, entity.Running, tstore, " hang tight 🤙")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspace, err = util.GetUserWorkspaceByNameOrIDErr(tstore, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func pollUntil(s *spinner.Spinner, wsid string, state string, shellStore ShellStore, waitMsg string) error {
	isReady := false
	s.Suffix = waitMsg
	s.Start()
	for !isReady {
		time.Sleep(5 * time.Second)
		ws, err := shellStore.GetWorkspace(wsid)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		s.Suffix = waitMsg
		if ws.Status == state {
			isReady = true
		}
	}
	return nil
}
