// Package start is for starting Brev workspaces
package start

import (
	"fmt"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var (
	startLong    = "Start a Brev machine that's in a paused or off state"
	startExample = `
  brev start <existing_ws_name>
	`
)

type StartStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	StartWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
}

func NewCmdStart(t *terminal.Terminal, startStore StartStore, noLoginStartStore StartStore) *cobra.Command {
	var detached bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "start",
		DisableFlagsInUseLine: true,
		Short:                 "Start an existing instance that is stopped",
		Long:                  startLong,
		Example:               startExample,
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoOrPathOrNameOrID := ""
			if len(args) > 0 {
				repoOrPathOrNameOrID = args[0]
			}

			err := runStartWorkspace(t, StartOptions{
				RepoOrPathOrNameOrID: repoOrPathOrNameOrID,
				Detached:             detached,
			}, startStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&detached, "detached", "d", false, "run the command in the background instead of blocking the shell")
	return cmd
}

type StartOptions struct {
	RepoOrPathOrNameOrID string // workspace name or ID to start
	Detached             bool
}

func runStartWorkspace(t *terminal.Terminal, options StartOptions, startStore StartStore) error {
	user, err := startStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	didStart, err := maybeStartStoppedOrJoin(t, user, options, startStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if didStart {
		return nil
	}

	if options.RepoOrPathOrNameOrID == "" {
		return breverrors.NewValidationError("Please specify a workspace name or ID to start. Use 'brev create' to create a new workspace.")
	} else {
		return breverrors.NewValidationError(fmt.Sprintf("No workspace found with name or ID '%s'. Use 'brev create' to create a new workspace.", options.RepoOrPathOrNameOrID))
	}
}

func maybeStartStoppedOrJoin(t *terminal.Terminal, user *entity.User, options StartOptions, startStore StartStore) (bool, error) {
	org, err := startStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	workspaces, err := startStore.GetWorkspaceByNameOrID(org.ID, options.RepoOrPathOrNameOrID)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	userWorkspaces := store.FilterForUserWorkspaces(workspaces, user.ID)
	if len(userWorkspaces) > 0 {
		if len(userWorkspaces) > 1 {
			userWorkspaces = store.FilterNonFailedWorkspaces(userWorkspaces)
			if len(userWorkspaces) > 1 {
				return false, breverrors.NewValidationError(fmt.Sprintf("multiple instances found with id/name %s", options.RepoOrPathOrNameOrID))
			}
			if len(userWorkspaces) == 0 {
				return false, breverrors.NewValidationError(fmt.Sprintf("workspace with id/name %s is a failed workspace", options.RepoOrPathOrNameOrID))
			}
		}
		errr := startStopppedWorkspace(&userWorkspaces[0], startStore, t, options)
		if errr != nil {
			return false, breverrors.WrapAndTrace(errr)
		}
		return true, nil
	}

	return false, nil
}

func startStopppedWorkspace(workspace *entity.Workspace, startStore StartStore, t *terminal.Terminal, startOptions StartOptions) error {
	if workspace.Status != entity.Stopped {
		return breverrors.NewValidationError(fmt.Sprintf("Instance is not stopped status=%s", workspace.Status))
	}

	startedWorkspace, err := startStore.StartWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf(t.Yellow("Instance %s is starting. \nNote: this can take about a minute. Run 'brev ls' to check status\n\n", startedWorkspace.Name))

	// Don't poll and block the shell if detached flag is set
	if startOptions.Detached {
		return nil
	}

	err = pollUntil(t, workspace.ID, entity.Running, startStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Print("\n")
	t.Vprint(t.Green("Your instance is ready!\n"))
	displayConnectBreadCrumb(t, startedWorkspace)

	return nil
}

func displayConnectBreadCrumb(t *terminal.Terminal, workspace *entity.Workspace) {
	t.Vprintf(t.Green("Connect to the instance:\n"))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev open %s\t# brev open <NAME> -> open instance in VS Code\n", workspace.Name)))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev shell %s\t# brev shell <NAME> -> ssh into instance (shortcut)\n", workspace.Name)))
	// t.Vprintf(t.Yellow(fmt.Sprintf("\tssh %s\t# ssh <SSH-NAME> -> ssh directly to instance\n", workspace.GetLocalIdentifier())))
}

func pollUntil(t *terminal.Terminal, wsid string, state string, startStore StartStore, canSafelyExit bool) error {
	s := t.NewSpinner()
	isReady := false
	if canSafelyExit {
		t.Vprintf("You can safely ctrl+c to exit\n")
	}
	s.Suffix = " hang tight 🤙"
	s.Start()
	for !isReady {
		time.Sleep(5 * time.Second)
		ws, err := startStore.GetWorkspace(wsid)
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
