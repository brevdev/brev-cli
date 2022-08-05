package shell

import (
	"os"
	"os/exec"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	openLong    = "[command in beta] This will shell in to your workspace"
	openExample = "brev shell workspace_id_or_name\nbrev shell my-app\nbrev open h9fp5vxwe"
)

type ShellStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	refresh.RefreshStore
}

func NewCmdShell(_ *terminal.Terminal, store ShellStore) *cobra.Command {
	var runRemoteCMD bool
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "shell",
		DisableFlagsInUseLine: true,
		Short:                 "[beta] open a shell in your dev environment",
		Long:                  openLong,
		Example:               openExample,
		Args:                  cmderrors.TransformToValidationError(cmderrors.TransformToValidationError(cobra.ExactArgs(1))),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runShellCommand(store, args[0], runRemoteCMD)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&runRemoteCMD, "remote", "r", true, "run remote commands")

	return cmd
}

func runShellCommand(sstore ShellStore, workspaceNameOrID string, runRemoteCMD bool) error {
	res := refresh.RunRefreshAsync(sstore, runRemoteCMD)

	workspace, err := util.GetUserWorkspaceByNameOrIDErr(sstore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	sshName := string(workspace.GetLocalIdentifier())

	err = res.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = runSSH(sshName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func runSSH(sshAlias string) error {
	sshCmd := exec.Command("ssh", sshAlias)
	sshCmd.Stderr = os.Stderr
	sshCmd.Stdout = os.Stdout
	sshCmd.Stdin = os.Stdin
	err := sshCmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
