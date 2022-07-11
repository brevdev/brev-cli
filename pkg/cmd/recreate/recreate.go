// Package recreate is for the recreate command
package recreate

import (
	_ "embed"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	stripmd "github.com/writeas/go-strip-markdown"
)

//go:embed doc.md
var long string

type recreateStore interface {
	completions.CompletionStore
	util.GetWorkspaceByNameOrIDErrStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	DeleteWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdRecreate(t *terminal.Terminal, store recreateStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "recreate",
		DisableFlagsInUseLine: true,
		Short:                 "TODO",
		Long:                  stripmd.Strip(long),
		Example:               "TODO",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunRecreate(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func RunRecreate(t *terminal.Terminal, args []string, recreateStore recreateStore) error {
	for _, arg := range args {
		err := hardResetProcess(arg, t, recreateStore)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

// hardResetProcess deletes an existing workspace and creates a new one
func hardResetProcess(workspaceName string, t *terminal.Terminal, recreateStore recreateStore) error {
	t.Vprint(t.Green("recreating 🤙 " + t.Yellow("This can take a couple of minutes.\n")))
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(recreateStore, workspaceName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	deletedWorkspace, err := recreateStore.DeleteWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Yellow("Deleting workspace - %s.", deletedWorkspace.Name))
	time.Sleep(10 * time.Second)

	if len(deletedWorkspace.GitRepo) != 0 {
		err := hardResetCreateWorkspaceFromRepo(t, recreateStore, deletedWorkspace)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		err := hardResetCreateEmptyWorkspace(t, recreateStore, deletedWorkspace)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

// hardResetCreateWorkspaceFromRepo clone a GIT repository, triggeres from the --hardreset flag
func hardResetCreateWorkspaceFromRepo(t *terminal.Terminal, recreateStore recreateStore, workspace *entity.Workspace) error {
	t.Vprint(t.Green("Workspace is starting. ") + t.Yellow("This can take up to 2 minutes the first time."))
	var orgID string
	activeorg, err := recreateStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if activeorg == nil {
		return breverrors.NewValidationError("no org exist")
	}
	orgID = activeorg.ID
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	options := store.NewCreateWorkspacesOptions(clusterID, workspace.Name).WithGitRepo(workspace.GitRepo)

	user, err := recreateStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	options = resolveWorkspaceUserOptions(options, user)

	options.StartupScriptPath = workspace.StartupScriptPath
	options.Execs = workspace.Execs
	options.Repos = workspace.Repos
	options.IDEConfig = &workspace.IDEConfig

	w, err := recreateStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, entity.Running, recreateStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour workspace is ready!"))
	t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.GetLocalIdentifier()))
	return nil
}

// hardResetCreateEmptyWorkspace creates a new empty worksapce,  triggered from the --hardreset flag
func hardResetCreateEmptyWorkspace(t *terminal.Terminal, recreateStore recreateStore, workspace *entity.Workspace) error {
	t.Vprint(t.Green("Workspace is starting. ") + t.Yellow("This can take up to 2 minutes the first time.\n"))

	// ensure name
	if len(workspace.Name) == 0 {
		return breverrors.NewValidationError("name field is required for empty workspaces")
	}

	// ensure org
	var orgID string
	activeorg, err := recreateStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if activeorg == nil {
		return breverrors.NewValidationError("no org exist")
	}
	orgID = activeorg.ID
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	options := store.NewCreateWorkspacesOptions(clusterID, workspace.Name)

	user, err := recreateStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	options = resolveWorkspaceUserOptions(options, user)

	options.StartupScriptPath = workspace.StartupScriptPath
	options.Execs = workspace.Execs
	options.Repos = workspace.Repos
	options.IDEConfig = &workspace.IDEConfig

	w, err := recreateStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, entity.Running, recreateStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour workspace is ready!"))
	t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.GetLocalIdentifier()))

	return nil
}

func pollUntil(t *terminal.Terminal, wsid string, state string, recreateStore recreateStore, canSafelyExit bool) error {
	s := t.NewSpinner()
	isReady := false
	if canSafelyExit {
		t.Vprintf("You can safely ctrl+c to exit\n")
	}
	s.Suffix = " hang tight 🤙"
	s.Start()
	for !isReady {
		time.Sleep(5 * time.Second)
		ws, err := recreateStore.GetWorkspace(wsid)
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
