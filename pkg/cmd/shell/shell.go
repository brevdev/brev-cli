package shell

import (
	"os"
	"os/exec"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
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
	openLong    = "[command in beta] This will shell in to your workspace"
	openExample = "brev shell workspace_id_or_name\nbrev shell my-app\nbrev open h9fp5vxwe"
)

type ShellStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	refresh.RefreshStore
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
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
			err := runShellCommand(store, args[0])
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&runRemoteCMD, "remote", "r", true, "run remote commands")

	return cmd
}

func runShellCommand(sstore ShellStore, workspaceNameOrID string) error {
	res := refresh.RunRefreshAsync(sstore)

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
