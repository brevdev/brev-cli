package shell

import (
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
	openLong    = "[command in beta] This will shell in to your workspace"
	openExample = "brev shell workspace_id_or_name\nbrev shell my-app\nbrev open h9fp5vxwe"
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
	return newCmdShellWithDeps(t, store, noLoginStartStore, exec.Command)
}

func newCmdShellWithDeps(t *terminal.Terminal, store ShellStore, noLoginStartStore ShellStore, executor cmdExecutor) *cobra.Command {
	var runRemoteCMD bool
	var directory string
	var host bool
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "shell",
		Aliases:               []string{"ssh"},
		DisableFlagsInUseLine: true,
		Short:                 "[beta] open a shell in your instance",
		Long:                  openLong,
		Example:               openExample,
		Args:                  cmderrors.TransformToValidationError(cmderrors.TransformToValidationError(cobra.ExactArgs(1))),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if SSH client is available
			if err := checkSSHClient(executor); err != nil {
				return breverrors.WrapAndTrace(err)
			}

			err := runShellCommand(t, store, args[0], directory, host)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&host, "host", "", false, "ssh into the host machine instead of the container")
	cmd.Flags().BoolVarP(&runRemoteCMD, "remote", "r", true, "run remote commands")
	cmd.Flags().StringVarP(&directory, "dir", "d", "", "override directory to launch shell")

	return cmd
}

func runShellCommand(t *terminal.Terminal, sstore ShellStore, workspaceNameOrID, directory string, host bool) error {
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
		return breverrors.New("Workspace is not running")
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
	err = runSSH(workspace, sshName, directory)
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

type cmdExecutor func(name string, arg ...string) *exec.Cmd

// checkSSHClient checks if the SSH client is available on the system.
func checkSSHClient(executor cmdExecutor) error {
	cmd := executor("ssh", "-V")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SSH client is not installed or not available in PATH. Please install SSH client to use this feature")
	}
	return nil
}

func waitForSSHToBeAvailable(sshAlias string, s *spinner.Spinner) error {
	counter := 0
	s.Suffix = " waiting for SSH connection to be available"
	s.Start()
	for {
		cmd := exec.Command("ssh", "-o", "ConnectTimeout=3", sshAlias, "echo", " ")
		out, err := cmd.CombinedOutput()
		if err == nil {
			s.Stop()
			return nil
		}

		outputStr := string(out)
		stdErr := strings.Split(outputStr, "\n")
		var stdErrMessage string
		if len(stdErr) > 1 {
			stdErrMessage = stdErr[1]
		} else {
			stdErrMessage = outputStr // Fallback if splitting fails
		}

		if counter == 120 || !store.SatisfactorySSHErrMessage(stdErrMessage) {
			return breverrors.WrapAndTrace(errors.New("\n" + stdErrMessage))
		}

		counter++
		time.Sleep(1 * time.Second)
	}
}

func runSSH(_ *entity.Workspace, sshAlias, _ string) error {
	sshAgentEval := "eval $(ssh-agent -s)"
	cmd := fmt.Sprintf("ssh %s", sshAlias)
	cmd = fmt.Sprintf("%s && %s", sshAgentEval, cmd)
	sshCmd := exec.Command("bash", "-c", cmd) //nolint:gosec //cmd is user input
	sshCmd.Stderr = os.Stderr
	sshCmd.Stdout = os.Stdout
	sshCmd.Stdin = os.Stdin

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
	t.Vprintf(t.Yellow("Instance %s is starting. \n\n", startedWorkspace.Name))
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
	defer s.Stop()
	for !isReady {
		ws, err := shellStore.GetWorkspace(wsid)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		s.Suffix = waitMsg
		if ws.Status == state {
			isReady = true
		}

		time.Sleep(5 * time.Second)
	}
	return nil
}
