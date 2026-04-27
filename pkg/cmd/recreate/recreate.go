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

const (
	createRetryWindow = 5 * time.Minute
	createRetryDelay  = 10 * time.Second
)

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

	t.Vprint(t.Yellow("Deleting instance - %s.", deletedWorkspace.Name))

	t.Vprint(t.Yellow("Waiting for delete to complete before creating new instance...\n"))
	time.Sleep(10 * time.Second)

	createErr := runCreateWithRetry(t, recreateStore, deletedWorkspace)
	if createErr != nil {
		return createErr
	}
	return nil
}

// isRetriableCreateError returns true for errors that may succeed on retry.
func isRetriableCreateError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "unexpected EOF") || strings.Contains(msg, "duplicate workspace") {
		return true
	}
	return false
}

func runCreateWithRetry(t *terminal.Terminal, recreateStore recreateStore, deletedWorkspace *entity.Workspace) error {
	deadline := time.Now().Add(createRetryWindow)
	var lastErr error
	for time.Now().Before(deadline) {
		if lastErr != nil {
			time.Sleep(createRetryDelay)
		}
		if len(deletedWorkspace.GitRepo) != 0 {
			lastErr = hardResetCreateWorkspaceFromRepo(t, recreateStore, deletedWorkspace)
		} else {
			lastErr = hardResetCreateEmptyWorkspace(t, recreateStore, deletedWorkspace)
		}
		if lastErr == nil {
			return nil
		}
		if !isRetriableCreateError(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

// hardResetCreateWorkspaceFromRepo clone a GIT repository, triggeres from the --hardreset flag
func hardResetCreateWorkspaceFromRepo(t *terminal.Terminal, recreateStore recreateStore, workspace *entity.Workspace) error {
	t.Vprint(t.Green("Instance is starting. ") + t.Yellow("This can take up to 2 minutes the first time."))
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
	if workspace.WorkspaceGroupID != "" {
		options = options.WithWorkspaceGroupID(workspace.WorkspaceGroupID)
	}
	if workspace.InstanceType != "" {
		options = options.WithInstanceType(workspace.InstanceType)
	}

	user, err := recreateStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	options = resolveWorkspaceUserOptions(options, user)

	options.StartupScriptPath = workspace.StartupScriptPath
	options.Execs = workspace.ExecsV0
	options.Repos = workspace.ReposV0
	options.IDEConfig = &workspace.IDEConfig

	w, err := recreateStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, entity.Running, recreateStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour instance is ready!"))
	t.Vprintf("%s", t.Green("\nSSH into your machine:\n\tssh %s\n", w.GetLocalIdentifier()))
	return nil
}

// hardResetCreateEmptyWorkspace creates a new empty worksapce,  triggered from the --hardreset flag
func hardResetCreateEmptyWorkspace(t *terminal.Terminal, recreateStore recreateStore, workspace *entity.Workspace) error {
	t.Vprint(t.Green("Instance is starting. ") + t.Yellow("This can take up to 2 minutes the first time.\n"))

	// ensure name
	if len(workspace.Name) == 0 {
		return breverrors.NewValidationError("name field is required for empty instances")
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
	if workspace.WorkspaceGroupID != "" {
		options = options.WithWorkspaceGroupID(workspace.WorkspaceGroupID)
	}
	if workspace.InstanceType != "" {
		options = options.WithInstanceType(workspace.InstanceType)
	}

	user, err := recreateStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	options = resolveWorkspaceUserOptions(options, user)

	options.StartupScriptPath = workspace.StartupScriptPath
	options.Execs = workspace.ExecsV0
	options.Repos = workspace.ReposV0
	options.IDEConfig = &workspace.IDEConfig

	w, err := recreateStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, entity.Running, recreateStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour instance is ready!"))
	t.Vprintf("%s", t.Green("\nSSH into your machine:\n\tssh %s\n", w.GetLocalIdentifier()))

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
		s.Suffix = "  instance is " + strings.ToLower(ws.Status)
		if ws.Status == state {
			s.Suffix = "Instance is ready!"
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
