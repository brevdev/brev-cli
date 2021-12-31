package test

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	startLong    = "[internal] test"
	startExample = "[internal] test"
)

type TestStore interface {
	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
}

func NewCmdTest(t *terminal.Terminal, store TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		Run: func(cmd *cobra.Command, args []string) {
			t.Vprint(args[0] + "\n")
			wsmeta, err := getWorkspaceFromNameOrID(args[0], store)
			if err != nil {
				t.Vprint(err.Error())
			}

			t.Vprintf("%s %s %s", wsmeta.Name, wsmeta.DNS, wsmeta.ID)

			// var err error

			// // SSH Keys
			// terminal.DisplayBrevLogo(t)
			// t.Vprintf("\n")
			// spinner := t.NewSpinner()
			// spinner.Suffix = " fetching your public key"
			// spinner.Start()
			// terminal.DisplaySSHKeys(t, "blahhhh fake key")
			// spinner.Stop()
			// if err != nil {
			// 	t.Vprintf(t.Red(err.Error()))
			// }

			// hasVSCode := terminal.PromptSelectInput(terminal.PromptSelectContent{
			// 	Label:    "Do you use VS Code?",
			// 	ErrorMsg: "error",
			// 	Items:    []string{"yes", "no"},
			// })
			// if hasVSCode == "yes" {
			// 	// TODO: check if user uses VSCode and intall extension for user
			// 	t.Vprintf(t.Yellow("Please install the following VS Code extension: ms-vscode-remote.remote-ssh\n"))
			// }

			// brevapi.InstallVSCodeExtension(t)

			// NOTE: this only works on Mac
			// err = beeep.Notify("Title", "Message body", "assets/information.png")
			// t.Vprintf("we just setn the beeeep")
			// if err != nil {
			// 	fmt.Println(err)
			// }

			// err = beeep.Beep(beeep.DefaultFreq, beeep.DefaultDuration)
			// if err != nil {
			// 	fmt.Println(err)
			// }

			// err = beeep.Alert("Title", "Message body", "assets/warning.png")
			// if err != nil {
			// 	fmt.Println(err)
			// }
		},
	}

	return cmd
}

func getWorkspaceFromNameOrID(nameOrID string, sstore TestStore) (*entity.WorkspaceWithMeta, error) {
	// Get Active Org
	org, err := sstore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, fmt.Errorf("no orgs exist")
	}

	// Get Current User
	currentUser, err := sstore.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// Get Workspaces for User
	var workspace *entity.Workspace // this will be the returned workspace
	workspaces, err := sstore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{Name: nameOrID, UserID: currentUser.ID})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if len(workspaces) == 0 {
		// In this case, check workspace by ID
		wsbyid, err := sstore.GetWorkspace(nameOrID) // Note: workspaceName is ID in this case
		if err != nil {
			return nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
		}
		if wsbyid != nil {
			workspace = wsbyid
		} else {
			// Can this case happen?
			return nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
		}
	} else if len(workspaces) > 1 {
		return nil, fmt.Errorf("multiple workspaces found with name %s\n\nTry running the command by id instead of name:\n\tbrev command <id>", nameOrID)
	} else {
		workspace = &workspaces[0]
	}

	if workspace == nil {
		return nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
	}

	// Get WorkspaceMetaData
	workspaceMetaData, err := sstore.GetWorkspaceMetaData(workspace.ID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &entity.WorkspaceWithMeta{WorkspaceMetaData: *workspaceMetaData, Workspace: *workspace}, nil
}
