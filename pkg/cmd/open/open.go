package open

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/alessio/shellescape"
	"github.com/spf13/cobra"
)

var (
	openLong    = "[command in beta] This will open VS Code SSH-ed in to your workspace. You must have 'code' installed in your path."
	openExample = "brev open workspace_id_or_name\nbrev open my-app\nbrev open h9fp5vxwe"
)

type OpenStore interface {
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error)
}

func NewCmdOpen(t *terminal.Terminal, store OpenStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "open",
		DisableFlagsInUseLine: true,
		Short:                 "[beta] open VSCode to ",
		Long:                  openLong,
		Example:               openExample,
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runOpenCommand(t, store, args[0])
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

// Fetch workspace info, then open code editor
func runOpenCommand(t *terminal.Terminal, tstore OpenStore, wsIDOrName string) error {
	fmt.Println("finding your workspace...")

	workspace, err := util.GetWorkspaceByNameOrIDErr(tstore, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	projPath := workspace.GetProjectFolderPath()

	localIdentifier := workspace.GetLocalIdentifier()

	t.Vprintf(t.Yellow("\nOpening VS Code to %s 🤙\n", projPath))
	err = openVsCodeWithSSH(string(localIdentifier), projPath)
	if err != nil {
		if strings.Contains(err.Error(), `"code": executable file not found in $PATH`) {
			errMsg := `"code": executable file not found in $PATH\n\nadd 'code' to your $PATH to open VS Code from the terminal\n\texport PATH="/Applications/Visual Studio Code.app/Contents/Resources/app/bin:$PATH"`
			return errors.New(errMsg)
		}
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

// Opens code editor. Attempts to install code in path if not installed already
func openVsCodeWithSSH(sshAlias string, path string) error {
	vscodeString := fmt.Sprintf("vscode-remote://ssh-remote+%s%s", sshAlias, path)

	vscodeString = shellescape.QuoteCommand([]string{vscodeString})
	cmd := exec.Command("code", "--folder-uri", vscodeString) // #nosec G204
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
