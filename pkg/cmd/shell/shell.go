package shell

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/writeconnectionevent"

	"github.com/spf13/cobra"
)

var (
	openLong    = "[command in beta] This will shell in to your instance"
	openExample = `  # SSH into an instance by name or ID
  brev shell instance_id_or_name
  brev shell my-instance

  # Create a GPU instance and SSH into it
  brev shell $(brev create my-instance)

  # Create with specific GPU and connect
  brev shell $(brev search --gpu-name A100 | brev create ml-box)

  # SSH into the host machine instead of the container
  brev shell my-instance --host

  # For non-interactive command execution, use 'brev exec':
  brev exec my-instance "nvidia-smi"`
)

type ShellStore interface {
	util.WorkspaceStartStore
	refresh.RefreshStore
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
}

func NewCmdShell(t *terminal.Terminal, store ShellStore, noLoginStartStore ShellStore) *cobra.Command {
	var host bool
	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "shell <instance>",
		Aliases:               []string{"ssh"},
		DisableFlagsInUseLine: true,
		Short:                 "[beta] Open a shell in your instance",
		Long:                  openLong,
		Example:               openExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			instanceName := args[0]
			err := runShellCommand(t, store, instanceName, host)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&host, "host", "", false, "ssh into the host machine instead of the container")

	return cmd
}

const pollTimeout = 10 * time.Minute

func runShellCommand(t *terminal.Terminal, sstore ShellStore, workspaceNameOrID string, host bool) error {
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
	err = runSSH(sshName)
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

func runSSH(sshAlias string) error {
	sshAgentEval := "eval $(ssh-agent -s)"
	cmd := fmt.Sprintf("%s && ssh %s", sshAgentEval, sshAlias)

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
