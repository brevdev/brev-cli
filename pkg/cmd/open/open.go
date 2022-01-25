package open

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	openLong    = "[command in beta] This will open VS Code SSH-ed in to your workspace. You must have 'code' installed in your path."
	openExample = "brev open workspace_id_or_name\nbrev open my-app\nbrev open h9fp5vxwe"
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

func NewCmdOpen(t *terminal.Terminal, store TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "open",
		DisableFlagsInUseLine: true,
		Short:                 "[beta] open VSCode to ",
		Long:                  openLong,
		Example:               openExample,
		Args:                  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			getWorkspaceSSHName(t, store, args[0])
		},
	}

	return cmd
}

func getWorkspaceSSHName(t *terminal.Terminal, tstore TestStore, wsIDOrName string) {
	s := t.NewSpinner()
	s.Suffix = " finding your workspace"
	s.Start()

	workspace, workspaces, err := getWorkspaceFromNameOrIDAndReturnWorkspacesPlusWorkspace(wsIDOrName, tstore)
	if err != nil {
		t.Errprint(err, "")
	}

	// Get base repo path
	splitBySlash := strings.Split(workspace.GitRepo, "/")[1]
	repoPath := strings.Split(splitBySlash, ".git")[0]

	// note: intentional decision to just assume the parent folder and inner folder are the same
	vscodeString := fmt.Sprintf("vscode-remote://ssh-remote+%s/home/brev/workspace/%s", workspace.GetLocalIdentifier(*workspaces), repoPath)
	s.Stop()
	t.Vprintf(t.Yellow("\nOpening VS Code to %s 🤙\n", repoPath))

	// see https://github.com/securego/gosec/issues/106#issuecomment-269714902
	// G204: Audit use of command execution
	// I believe the intent of this rule is to assist when performing code
	// audits of Go source code. It may not necessarily be an issue that
	// introduces command injection but you may want to know which external
	// executables are being called by an application when performing a security
	// review.
	cmd := exec.Command("code", "--folder-uri", vscodeString) // #nosec G204
	err = cmd.Run()

	if err != nil {
		t.Errprint(err, "")
	}
}

// NOTE: this function is copy/pasted in many places. If you modify it, modify it elsewhere.
// Reasoning: there wasn't a utils file so I didn't know where to put it
//                + not sure how to pass a generic "store" object
// NOTE2: small modification on this one-- returning all the workspaces too in case they're needed and shouldn't be fetched twice.
func getWorkspaceFromNameOrIDAndReturnWorkspacesPlusWorkspace(nameOrID string, sstore TestStore) (*entity.WorkspaceWithMeta, *[]entity.Workspace, error) {
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
