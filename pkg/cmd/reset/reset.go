package reset

import (
	"fmt"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	startLong    = "Reset your machine if it's acting up. This deletes the machine and gets you a fresh one."
	startExample = `  brev reset <ws_name>
  brev reset <ws_name> --hard`
)

type ResetStore interface {
	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	DeleteWorkspace(workspaceID string) (*entity.Workspace, error)
	StartWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdReset(t *terminal.Terminal, loginResetStore ResetStore, noLoginResetStore ResetStore) *cobra.Command {
	var hardreset bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "reset",
		DisableFlagsInUseLine: true,
		Short:                 "Reset a workspace if it's in a weird state.",
		Long:                  startLong,
		Example:               startExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginResetStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			if hardreset {
				err := hardResetProcess(args[0], t, loginResetStore)
				if err != nil {
					t.Vprint(t.Red(err.Error()))
				}
			} else {
				err := resetWorkspace(args[0], t, loginResetStore)
				if err != nil {
					t.Vprint(t.Red(err.Error()))
				}

			}
		},
	}

	cmd.Flags().BoolVarP(&hardreset, "hard", "", false, "deletes the workspace and creates a fresh version WARNING: this is destructive and workspace state not tracked in git is lost")
	return cmd
}

// hardResetProcess deletes an existing workspace and creates a new one
func hardResetProcess(workspaceName string, t *terminal.Terminal, resetStore ResetStore) error {
	t.Vprint(t.Green("Starting hard reset 🤙 " + t.Yellow("This can take a couple of minutes.\n")))
	workspace, err := getWorkspaceFromNameOrID(workspaceName, resetStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	deletedWorkspace, err := resetStore.DeleteWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Yellow("Deleting workspace - %s.", deletedWorkspace.Name))
	time.Sleep(10 * time.Second)

	if len(deletedWorkspace.GitRepo) != 0 {
		err := hardResetCreateWorkspaceFromRepo(t, resetStore, deletedWorkspace.Name, deletedWorkspace.GitRepo)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		err := hardResetCreateEmptyWorkspace(t, resetStore, deletedWorkspace.Name)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

// hardResetCreateWorkspaceFromRepo clone a GIT repository, triggeres from the --hardreset flag
func hardResetCreateWorkspaceFromRepo(t *terminal.Terminal, resetStore ResetStore, name, repo string) error {
	t.Vprint(t.Green("\nWorkspace is starting. ") + t.Yellow("This can take up to 2 minutes the first time.\n"))
	var orgID string
	activeorg, err := resetStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if activeorg == nil {
		return fmt.Errorf("no org exist")
	}
	orgID = activeorg.ID
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	options := store.NewCreateWorkspacesOptions(clusterID, name).WithGitRepo(repo)

	user, err := resetStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	options = resolveWorkspaceUserOptions(options, user)

	w, err := resetStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, "RUNNING", resetStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour workspace is ready!"))
	t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.GetLocalIdentifier()))
	return nil
}

// hardResetCreateEmptyWorkspace creates a new empty worksapce,  triggered from the --hardreset flag
func hardResetCreateEmptyWorkspace(t *terminal.Terminal, resetStore ResetStore, name string) error {
	t.Vprint(t.Green("\nWorkspace is starting. ") + t.Yellow("This can take up to 2 minutes the first time.\n"))

	// ensure name
	if len(name) == 0 {
		return fmt.Errorf("name field is required for empty workspaces")
	}

	// ensure org
	var orgID string
	activeorg, err := resetStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if activeorg == nil {
		return fmt.Errorf("no org exist")
	}
	orgID = activeorg.ID
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	options := store.NewCreateWorkspacesOptions(clusterID, name)

	user, err := resetStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	options = resolveWorkspaceUserOptions(options, user)

	w, err := resetStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, "RUNNING", resetStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour workspace is ready!"))
	t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.GetLocalIdentifier()))

	return nil
}

func pollUntil(t *terminal.Terminal, wsid string, state string, resetStore ResetStore, canSafelyExit bool) error {
	s := t.NewSpinner()
	isReady := false
	if canSafelyExit {
		t.Vprintf("You can safely ctrl+c to exit\n")
	}
	s.Suffix = " hang tight 🤙"
	s.Start()
	for !isReady {
		time.Sleep(5 * time.Second)
		ws, err := resetStore.GetWorkspace(wsid)
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

func resolveWorkspaceUserOptions(options *store.CreateWorkspacesOptions, user *entity.User) *store.CreateWorkspacesOptions {
	if options.WorkspaceTemplateID == "" {
		if featureflag.IsAdmin(user.GlobalUserType) {
			options.WorkspaceTemplateID = store.DevWorkspaceTemplateID
		} else {
			options.WorkspaceTemplateID = store.UserWorkspaceTemplateID
		}
	}
	if options.WorkspaceClassID == "" {
		if featureflag.IsAdmin(user.GlobalUserType) {
			options.WorkspaceClassID = store.DevWorkspaceClassID
		} else {
			options.WorkspaceClassID = store.UserWorkspaceClassID
		}
	}
	return options
}

func resetWorkspace(workspaceName string, t *terminal.Terminal, resetStore ResetStore) error {
	workspace, err := getWorkspaceFromNameOrID(workspaceName, resetStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	startedWorkspace, err := resetStore.ResetWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("Workspace %s is resetting. \n Note: this can take a few seconds. Run 'brev ls' to check status\n", startedWorkspace.Name)

	return nil
}

// NOTE: this function is copy/pasted in many places. If you modify it, modify it elsewhere.
// Reasoning: there wasn't a utils file so I didn't know where to put it
//                + not sure how to pass a generic "store" object
func getWorkspaceFromNameOrID(nameOrID string, sstore ResetStore) (*entity.WorkspaceWithMeta, error) {
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

	switch len(workspaces) {
	case 0:
		// In this case, check workspace by ID
		wsbyid, othererr := sstore.GetWorkspace(nameOrID) // Note: workspaceName is ID in this case
		if othererr != nil {
			return nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
		}
		if wsbyid != nil {
			workspace = wsbyid
		} else {
			// Can this case happen?
			return nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
		}
	case 1:
		workspace = &workspaces[0]
	default:
		return nil, fmt.Errorf("multiple workspaces found with name %s\n\nTry running the command by id instead of name:\n\tbrev command <id>", nameOrID)
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
