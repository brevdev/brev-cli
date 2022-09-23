package open

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
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

	"github.com/alessio/shellescape"
	"github.com/spf13/cobra"
)

var (
	openLong    = "[command in beta] This will open VS Code SSH-ed in to your workspace. You must have 'code' installed in your path."
	openExample = "brev open workspace_id_or_name\nbrev open my-app\nbrev open h9fp5vxwe"
)

type OpenStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	refresh.RefreshStore
	UpdateUser(string, *entity.UpdateUser) (*entity.User, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	StartWorkspace(workspaceID string) (*entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdOpen(t *terminal.Terminal, store OpenStore, noLoginStartStore OpenStore) *cobra.Command {
	var runRemotCMD bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "open",
		DisableFlagsInUseLine: true,
		Short:                 "[beta] open VSCode to ",
		Long:                  openLong,
		Example:               openExample,
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runOpenCommand(t, store, args[0], runRemotCMD)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&runRemotCMD, "remote", "r", true, "run remote command")

	return cmd
}

func pollUntil(t *terminal.Terminal, wsid string, state string, openStore OpenStore) error {
	s := t.NewSpinner()
	isReady := false
	s.Suffix = " hang tight 🤙"
	s.Start()
	for !isReady {
		time.Sleep(5 * time.Second)
		ws, err := openStore.GetWorkspace(wsid)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		s.Suffix = "  workspace is " + strings.ToLower(ws.Status)
		if ws.Status == state {
			s.Suffix = "Workspace is ready!"
			s.Stop()
			isReady = true
		}
	}
	return nil
}

// Fetch workspace info, then open code editor
func runOpenCommand(t *terminal.Terminal, tstore OpenStore, wsIDOrName string, runRemoteCMD bool) error {
	// todo check if workspace is stopped and start if it if it is stopped
	fmt.Println("finding your dev environment...")

	res := refresh.RunRefreshAsync(tstore, runRemoteCMD)

	workspace, err := util.GetUserWorkspaceByNameOrIDErr(tstore, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspace.Status == "STOPPED" { // we start the env for the user
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
	}

	if workspace.Status != "RUNNING" {
		return breverrors.WorkspaceNotRunning{Status: workspace.Status}
	}

	projPath := workspace.GetProjectFolderPath()

	localIdentifier := workspace.GetLocalIdentifier()

	t.Vprintf(t.Yellow("\nOpening VSCode to %s 🤙\n", projPath))

	err = res.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = hello.SetHasRunOpen(true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = openVsCodeWithSSH(string(localIdentifier), projPath)
	if err != nil {
		if strings.Contains(err.Error(), `"code": executable file not found in $PATH`) {
			errMsg := "code\": executable file not found in $PATH\n\nadd 'code' to your $PATH to open VS Code from the terminal\n\texport PATH=\"/Applications/Visual Studio Code.app/Contents/Resources/app/bin:$PATH\""
			_, errStore := tstore.UpdateUser(
				workspace.CreatedByUserID,
				&entity.UpdateUser{
					OnboardingData: map[string]interface{}{
						"pathErrorTS": time.Now().UTC().Unix(),
					},
				})
			if errStore != nil {
				return errors.New(errMsg + "\n" + errStore.Error())
			}
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
