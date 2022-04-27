package open

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

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

// Fetch workspace info, then open code editor
func runOpenCommand(t *terminal.Terminal, tstore OpenStore, wsIDOrName string) error {
	s := t.NewSpinner()
	s.Suffix = " finding your workspace"
	s.Start()

	workspace, workspaces, err := getWorkspaceFromNameOrIDAndReturnWorkspacesPlusWorkspace(wsIDOrName, tstore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Get the folder name
	// Get base repo path or use workspace Name
	var folderName string
	if len(workspace.GitRepo) > 0 {
		splitBySlash := strings.Split(workspace.GitRepo, "/")[1]
		repoPath := strings.Split(splitBySlash, ".git")[0]
		folderName = repoPath
	} else {
		folderName = workspace.Name
	}

	// note: intentional decision to just assume the parent folder and inner folder are the same
	s.Stop()
	localIdentifier := workspace.GetLocalIdentifier(*workspaces)

	err = openCode(string(localIdentifier), folderName, t)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

// Opens code editor. Attempts to install code in path if not installed already
func openCode(localIdentifier string, folderName string, t *terminal.Terminal) error {
	vscodeString := fmt.Sprintf("vscode-remote://ssh-remote+%s/home/brev/workspace/%s", localIdentifier, folderName)
	t.Vprintf(t.Yellow("\nOpening VS Code to %s 🤙\n", folderName))

	vscodeString = shellescape.QuoteCommand([]string{vscodeString})
	cmd := exec.Command("code", "--folder-uri", vscodeString) // #nosec G204
	err := cmd.Run()
	if err != nil {
		if strings.Contains(err.Error(), `"code": executable file not found in $PATH`) {
			errString := t.Red("\n\nYou need to add 'code' to your $PATH to open VS Code from the terminal.\n") + t.Yellow("Try this command:\n") + "\t" + t.Yellow(`export PATH="/Applications/Visual Studio Code.app/Contents/Resources/app/bin:$PATH"`) + "\n\n"

			// NOTE: leaving this here. I tried to add code to path for them automatically but that was difficult bcs go != their shell
			// _, err2 := os.Stat("/Applications/Visual Studio Code.app/Contents/Resources/app/bin")
			// if os.IsNotExist(err2) {
			// 	// TODO: have them add manually
			// 	return breverrors.WrapAndTrace(err)
			// } else {
			// 	// TODO: add it for them since you can access the application
			// 	// export PATH="/Applications/Visual Studio Code.app/Contents/Resources/app/bin:$PATH"

			// 	cmd = exec.Command("sh", "-c", `export PATH="/Applications/Visual Studio Code.app/Contents/Resources/app/bin:$PATH"`)
			// 	errPath := cmd.Run()
			// 	if errPath != nil {
			// 		t.Vprintf("error adding to path: ", errPath.Error())
			// 	}

			// 	t.Vprintf("Folder does exist.")
			// }

			return errors.New(errString)
		}

		return breverrors.WrapAndTrace(err)
	}

	return nil
}

// NOTE: this function is copy/pasted in many places. If you modify it, modify it elsewhere.
// Reasoning: there wasn't a utils file so I didn't know where to put it
//                + not sure how to pass a generic "store" object
// NOTE2: small modification on this one-- returning all the workspaces too in case they're needed and shouldn't be fetched twice.
func getWorkspaceFromNameOrIDAndReturnWorkspacesPlusWorkspace(nameOrID string, sstore OpenStore) (*entity.WorkspaceWithMeta, *[]entity.Workspace, error) {
	// Get Active Org
	org, err := sstore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, nil, fmt.Errorf("no orgs exist")
	}

	// Get Current User
	currentUser, err := sstore.GetCurrentUser()
	if err != nil {
		return nil, nil, breverrors.WrapAndTrace(err)
	}

	// Get Workspaces for User
	var workspace *entity.Workspace // this will be the returned workspace
	workspaces, err := sstore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{Name: nameOrID, UserID: currentUser.ID})
	if err != nil {
		return nil, nil, breverrors.WrapAndTrace(err)
	}

	switch len(workspaces) {
	case 0:
		// In this case, check workspace by ID
		wsbyid, othererr := sstore.GetWorkspace(nameOrID) // Note: workspaceName is ID in this case
		if othererr != nil {
			return nil, nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
		}
		if wsbyid != nil {
			workspace = wsbyid
		} else {
			// Can this case happen?
			return nil, nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
		}
	case 1:
		workspace = &workspaces[0]
	default:
		return nil, nil, fmt.Errorf("multiple workspaces found with name %s\n\nTry running the command by id instead of name:\n\tbrev command <id>", nameOrID)
	}

	if workspace == nil {
		return nil, nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
	}

	// Get WorkspaceMetaData
	workspaceMetaData, err := sstore.GetWorkspaceMetaData(workspace.ID)
	if err != nil {
		return nil, nil, breverrors.WrapAndTrace(err)
	}

	return &entity.WorkspaceWithMeta{WorkspaceMetaData: *workspaceMetaData, Workspace: *workspace}, &workspaces, nil
}
