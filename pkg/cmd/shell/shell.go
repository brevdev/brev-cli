package shell

import (
	"fmt"
	"os"
	"os/exec"
	"time"

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
	var runRemoteCMD bool
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "shell",
		DisableFlagsInUseLine: true,
		Short:                 "[beta] open a shell in your dev environment",
		Long:                  openLong,
		Example:               openExample,
		Args:                  cmderrors.TransformToValidationError(cmderrors.TransformToValidationError(cobra.ExactArgs(1))),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runShellCommand(t, store, args[0])
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&runRemoteCMD, "remote", "r", true, "run remote commands")

	return cmd
}

func runShellCommand(t *terminal.Terminal, sstore ShellStore, workspaceNameOrID string) error {
	res := refresh.RunRefreshAsync(sstore)

	workspace, err := util.GetUserWorkspaceByNameOrIDErr(sstore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if workspace.Status == "STOPPED" { // we start the env for the user
		err = startWorkspaceIfStopped(t, sstore, workspaceNameOrID, workspace)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	res = refresh.RunRefreshAsync(sstore)
	workspace, err = util.GetUserWorkspaceByNameOrIDErr(sstore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspace.Status != "RUNNING" {
		return breverrors.New("Workspace is not running")
	}
	sshName := string(workspace.GetLocalIdentifier())

	err = res.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	waitForSSHToBeAvailable(sshName)
	// we don't care about the error here but should log with sentry
	// legacy environments wont support this and cause errrors, 
	// but we don't want to block the user from using the shell
	_ = writeconnectionevent.WriteWCEOnEnv(sstore, workspace.DNS) 
	err = runSSH(workspace, sshName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func waitForSSHToBeAvailable(sshAlias string) {
	counter := 0
	for {
		cmd := exec.Command("ssh", sshAlias, "echo", " ")
		_, err := cmd.CombinedOutput()
		if err == nil {
			return
		}
		if counter == 1 {
			fmt.Println("\nPerforming final checks...")
		}
		counter++
		time.Sleep(1 * time.Second)
	}
}

func runSSH(workspace *entity.Workspace, sshAlias string) error {
	sshCmd := exec.Command("ssh", sshAlias)
	if workspace.GetProjectFolderPath() != "" {
		sshCmd = exec.Command("ssh", "-t", sshAlias, "cd", workspace.GetProjectFolderPath(), "&&", "zsh") //nolint:gosec //this is being run on a user's machine so we're not concerned about shell injection
	}
	sshCmd.Stderr = os.Stderr
	sshCmd.Stdout = os.Stdout
	sshCmd.Stdin = os.Stdin

	// BANANA: there's probably a better place for this.
	// 		persistentPOSTrun for this function never gets called...
	// 		I could set this to the prerun, but then it happens before they ssh in
	// 		....
	// This boolean tells the onboarding to continue to the next step!
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

func startWorkspaceIfStopped(t *terminal.Terminal, tstore ShellStore, wsIDOrName string, workspace *entity.Workspace) error {
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
	t.Vprintf(t.Yellow("Dev environment %s is starting. \n\n", startedWorkspace.Name))
	err = pollUntil(t, workspace.ID, entity.Running, tstore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspace, err = util.GetUserWorkspaceByNameOrIDErr(tstore, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func pollUntil(t *terminal.Terminal, wsid string, state string, shellStore ShellStore) error {
	s := t.NewSpinner()
	isReady := false
	s.Suffix = " hang tight 🤙"
	s.Start()
	for !isReady {
		time.Sleep(5 * time.Second)
		ws, err := shellStore.GetWorkspace(wsid)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		s.Suffix = " hang tight 🤙"
		if ws.Status == state {
			s.Suffix = "Workspace is ready!"
			s.Stop()
			isReady = true
		}
	}
	return nil
}
