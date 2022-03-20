package open

import (
	"os/exec"

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
	GetVSCodeOpenString(wsIDOrName string) (string, string, error)
}

func NewCmdOpen(t *terminal.Terminal, store OpenStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "open",
		DisableFlagsInUseLine: true,
		Short:                 "[beta] open VSCode to ",
		Long:                  openLong,
		Example:               openExample,
		Args:                  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := runOpenCommand(t, store, args[0])
			if err != nil {
				t.Errprint(err, "")
			}
		},
	}

	return cmd
}

func runOpenCommand(t *terminal.Terminal, tstore OpenStore, wsIDOrName string) error {
	s := t.NewSpinner()
	s.Suffix = " finding your workspace"
	s.Start()

	vsCodeString, folderName, err := tstore.GetVSCodeOpenString(wsIDOrName)
	if err != nil {
		return err
	}
	s.Stop()
	t.Vprintf(t.Yellow("\nOpening VS Code to %s 🤙\n", folderName))

	vsCodeString = shellescape.QuoteCommand([]string{vsCodeString})
	cmd := exec.Command("code", "--folder-uri", vsCodeString) // #nosec G204
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
