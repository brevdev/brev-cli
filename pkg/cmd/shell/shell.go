package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
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
	GetAccessToken() (string, error)
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
	if _, err := sstore.GetAccessToken(); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	s := t.NewSpinner()
	target, err := util.ResolveWorkspaceOrNode(sstore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if target.Node != nil {
		return shellIntoExternalNode(t, sstore, target.Node)
	}
	workspace := target.Workspace

	if workspace.Status == "STOPPED" { // we start the env for the user
		err = util.StartWorkspaceIfStopped(t, s, sstore, workspaceNameOrID, workspace, pollTimeout)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	if workspace.Status != "RUNNING" {
		err = util.PollUntil(s, workspace.ID, "RUNNING", sstore, " waiting for instance to be ready...", pollTimeout)
	}
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	localIdentifier := workspace.GetLocalIdentifier()
	if host {
		localIdentifier = workspace.GetHostIdentifier()
	}
	sshName := string(localIdentifier)

	err = runSSHWithOptions(sshName, host, false)
	if err == nil {
		trackShellAnalytics(sstore, workspace)
		return nil
	}
	_, _ = fmt.Fprintln(os.Stderr, "\nConnection failed, refreshing SSH config and retrying...")

	refreshRes := refresh.RunRefreshAsync(sstore)

	workspace, err = util.GetUserWorkspaceByNameOrIDErr(sstore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspace.Status != "RUNNING" {
		return breverrors.New("Instance is not running")
	}

	err = refreshRes.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = util.WaitForSSHToBeAvailable(sshName, s)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = runSSH(sshName, host)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	trackShellAnalytics(sstore, workspace)

	return nil
}

func trackShellAnalytics(sstore ShellStore, workspace *entity.Workspace) {
	userID := ""
	user, err := sstore.GetCurrentUser()
	if err != nil {
		userID = workspace.CreatedByUserID
	} else {
		userID = user.ID
	}
	data := analytics.EventData{
		EventName: "Brev Shell",
		UserID:    userID,
		Properties: map[string]string{
			"instanceId": workspace.ID,
		},
	}
	_ = analytics.TrackEvent(data)
}

func shellIntoExternalNode(t *terminal.Terminal, sstore ShellStore, node *nodev1.ExternalNode) error {
	info, err := util.ResolveExternalNodeSSH(sstore, node)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	privateKeyPath, err := sstore.GetPrivateKeyPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		t.Vprintf("fetching keys...\n")
		if refreshErr := refresh.RunRefreshAsync(sstore).Await(); refreshErr != nil {
			return breverrors.WrapAndTrace(refreshErr)
		}
	}

	t.Vprintf("Connecting to external node %q as %s on port %d (key: %s)...\n", node.GetName(), info.LinuxUser, info.Port, privateKeyPath)
	return runSSHWithPort(info.SSHTarget(), info.Port, privateKeyPath)
}

func runSSHWithPort(target string, port int32, identityFile string) error {
	sshAgentEval := `if [ -z "$SSH_AUTH_SOCK" ]; then eval $(ssh-agent -s) > /dev/null; fi`
	cmd := fmt.Sprintf("%s && ssh -i %q -o StrictHostKeyChecking=no -p %d %s", sshAgentEval, identityFile, port, target)

	sshCmd := exec.Command("bash", "-c", cmd) //nolint:gosec //cmd is constructed from API data
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

func runSSH(sshAlias string, host bool) error {
	return runSSHWithOptions(sshAlias, host, true)
}

func runSSHWithOptions(sshAlias string, host bool, printFailureAdvice bool) error {
	sshAgentEval := `if [ -z "$SSH_AUTH_SOCK" ]; then eval $(ssh-agent -s) > /dev/null; fi`
	var cmd string
	if host {
		cmd = fmt.Sprintf("%s && ssh -o ConnectTimeout=5 %s", sshAgentEval, sshAlias)
	} else {
		// SSH into VM and respect container WORKDIR if containerized, otherwise use default directory
		cmd = fmt.Sprintf("%s && ssh -t -o ConnectTimeout=5 %s 'DIR=$(readlink -f /proc/1/cwd 2>/dev/null || pwd); cd \"$DIR\" || echo \"Warning: Could not access container directory\" >&2; exec -l ${SHELL:-/bin/sh}'", sshAgentEval, sshAlias)
	}

	var stderrBuf bytes.Buffer
	sshCmd := exec.Command("bash", "-c", cmd) //nolint:gosec //cmd is user input
	sshCmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stdin = os.Stdin

	err := hello.SetHasRunShell(true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = sshCmd.Run()
	if err != nil {
		if !printFailureAdvice {
			return breverrors.WrapAndTrace(err)
		}
		stderrStr := stderrBuf.String()
		if strings.Contains(stderrStr, "unix_listener") || strings.Contains(stderrStr, "path too long") {
			fmt.Fprintf(os.Stderr, "\nbrev shell failed: SSH ControlPath socket path is too long for this system.\n")
			fmt.Fprintf(os.Stderr, "Fix: run 'brev refresh' to regenerate your SSH config with the updated ControlPath.\n")
		} else {
			fmt.Fprintf(os.Stderr, "\nbrev shell failed. If the above SSH error is unclear, try running 'brev refresh' and reconnecting.\n")
		}
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
