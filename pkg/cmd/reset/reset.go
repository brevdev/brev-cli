package reset

import (
	_ "embed"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/cobra"
	stripmd "github.com/writeas/go-strip-markdown"
)

var (
	//go:embed doc.md
	long         string
	startExample = `brev reset <ws_name>...`
)

type ResetStore interface {
	completions.CompletionStore
	util.GetWorkspaceByNameOrIDErrStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	DeleteWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdReset(t *terminal.Terminal, loginResetStore ResetStore, noLoginResetStore ResetStore) *cobra.Command {
	var hardreset bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "reset",
		DisableFlagsInUseLine: true,
		Short:                 "Reset a workspace if it's in a weird state.",
		Long:                  stripmd.Strip(long),
		Example:               startExample,
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginResetStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if hardreset {
					err := hardResetProcess(arg, t, loginResetStore)
					if err != nil {
						return breverrors.WrapAndTrace(err)
					}
				} else {
					err := resetWorkspace(arg, t, loginResetStore)
					if err != nil {
						return breverrors.WrapAndTrace(err)
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&hardreset, "hard", "", false, "DEPRECATED: use brev recreate")
	return cmd
}

// hardResetProcess deletes an existing workspace and creates a new one
func hardResetProcess(workspaceName string, t *terminal.Terminal, resetStore ResetStore) error {
	t.Vprint(t.Green("Starting hard reset 🤙 " + t.Yellow("This can take a couple of minutes.\n")))
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(resetStore, workspaceName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	deletedWorkspace, err := resetStore.DeleteWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Yellow("Deleting dev environment - %s.", deletedWorkspace.Name))
	time.Sleep(10 * time.Second)

	if len(deletedWorkspace.GitRepo) != 0 {
		err := hardResetCreateWorkspaceFromRepo(t, resetStore, deletedWorkspace)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		err := hardResetCreateEmptyWorkspace(t, resetStore, deletedWorkspace)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	t.Vprint(t.Red("NOTE: THIS COMMAND IS DEPRECATED"))
	t.Vprint(t.Red("It still worked, but use brev recreate " + workspaceName + " next time"))
	return nil
}

// hardResetCreateWorkspaceFromRepo clone a GIT repository, triggeres from the --hardreset flag
func hardResetCreateWorkspaceFromRepo(t *terminal.Terminal, resetStore ResetStore, workspace *entity.Workspace) error {
	t.Vprint(t.Green("Dev environment is starting. ") + t.Yellow("This can take up to 2 minutes the first time."))
	var orgID string
	activeorg, err := resetStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if activeorg == nil {
		return breverrors.NewValidationError("no org exist")
	}
	orgID = activeorg.ID
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	options := store.NewCreateWorkspacesOptions(clusterID, workspace.Name).WithGitRepo(workspace.GitRepo)

	user, err := resetStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	options = resolveWorkspaceUserOptions(options, user)

	options.StartupScriptPath = workspace.StartupScriptPath
	options.Execs = workspace.ExecsV0
	options.Repos = workspace.ReposV0
	options.IDEConfig = &workspace.IDEConfig

	w, err := resetStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, entity.Running, resetStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour dev environment is ready!"))
	t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.GetLocalIdentifier()))
	return nil
}

// hardResetCreateEmptyWorkspace creates a new empty worksapce,  triggered from the --hardreset flag
func hardResetCreateEmptyWorkspace(t *terminal.Terminal, resetStore ResetStore, workspace *entity.Workspace) error {
	t.Vprint(t.Green("Dev environment is starting. ") + t.Yellow("This can take up to 2 minutes the first time.\n"))

	// ensure name
	if len(workspace.Name) == 0 {
		return breverrors.NewValidationError("name field is required for empty dev environments")
	}

	// ensure org
	var orgID string
	activeorg, err := resetStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if activeorg == nil {
		return breverrors.NewValidationError("no org exist")
	}
	orgID = activeorg.ID
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	options := store.NewCreateWorkspacesOptions(clusterID, workspace.Name)

	user, err := resetStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	options = resolveWorkspaceUserOptions(options, user)

	options.StartupScriptPath = workspace.StartupScriptPath
	options.Execs = workspace.ExecsV0
	options.Repos = workspace.ReposV0
	options.IDEConfig = &workspace.IDEConfig

	w, err := resetStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, entity.Running, resetStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour dev environment is ready!"))
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
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(resetStore, workspaceName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	startedWorkspace, err := resetStore.ResetWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf(t.Yellow("Dev environment %s is resetting.\n", startedWorkspace.Name))
	t.Vprintf("Note: this can take a few seconds. Run 'brev ls' to check status\n")

	return nil
}
