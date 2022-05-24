package shell

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	openLong    = "[command in beta] This will shell in to your workspace"
	openExample = "brev shell workspace_id_or_name\nbrev shell my-app\nbrev open h9fp5vxwe"
)

type ShellStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error)
}

func NewCmdShell(_ *terminal.Terminal, store ShellStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "shell",
		DisableFlagsInUseLine: true,
		Short:                 "[beta] open a shell in your workspace",
		Long:                  openLong,
		Example:               openExample,
		Args:                  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runShellCommand(store, args[0])
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func runShellCommand(store ShellStore, workspaceNameOrID string) error {
	org, err := store.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaces, err := store.GetWorkspaceByNameOrID(org.ID, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(workspaces) == 0 {
		return breverrors.NewValidationError(fmt.Sprintf("workspace with id/name %s not found", workspaceNameOrID))
	}
	if len(workspaces) > 1 {
		return breverrors.NewValidationError(fmt.Sprintf("multiple workspaces found with id/name %s", workspaceNameOrID))
	}
	sshName := string(workspaces[0].GetLocalIdentifier())
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
