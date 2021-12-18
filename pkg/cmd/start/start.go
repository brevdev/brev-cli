// Package start is for starting Brev workspaces
package start

import (
	"fmt"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var (
	startLong    = "Start a Brev machine that's in a paused or off state or create one from a url"
	startExample = `
  brev start <existing_ws_name>
  brev start <git url>
  brev start <git url> --org myFancyOrg
	`
)

type StartStore interface {
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	StartWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
}

func NewCmdStart(t *terminal.Terminal, loginStartStore StartStore, noLoginStartStore StartStore) *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "start",
		DisableFlagsInUseLine: true,
		Short:                 "Start a workspace if it's stopped, or create one from url",
		Long:                  startLong,
		Example:               startExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			isURL := false
			if strings.Contains(args[0], "https://") || strings.Contains(args[0], "git@") {
				isURL = true
			}

			if !isURL {
				err := startWorkspace(args[0], loginStartStore, t)
				if err != nil {
					t.Vprint(t.Red(err.Error()))
				}
			} else {
				// CREATE A WORKSPACE
				err := clone(t, args[0], org, loginStartStore)
				if err != nil {
					t.Vprint(t.Red(err.Error()))
				}
			}
		},
	}
	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org if creating a workspace)")
	err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noLoginStartStore, t))
	if err != nil {
		t.Errprint(err, "cli err")
	}

	return cmd
}

func startWorkspace(workspaceName string, startStore StartStore, t *terminal.Terminal) error {
	org, err := startStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return fmt.Errorf("no orgs exist")
	}

	user, err := startStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspaces, err := startStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{
		UserID: user.ID,
		Name:   workspaceName,
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if len(workspaces) == 0 {
		return fmt.Errorf("no workspace found of name %s", workspaceName)
	} else if len(workspaces) > 1 {
		return fmt.Errorf("more than 1 workspace found of name %s", workspaceName)
	}

	workspace := workspaces[0]

	startedWorkspace, err := startStore.StartWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf(t.Yellow("\nWorkspace %s is starting. \nNote: this can take about a minute. Run 'brev ls' to check status\n", startedWorkspace.Name))

	t.Vprintf("You can safely ctrl+c to exit\n")

	err = pollUntil(t, workspace.ID, "RUNNING", startStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour workspace is ready!"))

	t.Vprintf(t.Green("\n\nTo connect to your machine, make sure to Brev up:") +
		t.Yellow("\n\t$ brev up\n"))

	t.Vprintf("\nIf you have 'brev up' running, SSH into your machine:\n\tssh %s\n", workspace.Name)

	return nil
}

// "https://github.com/brevdev/microservices-demo.git
// "https://github.com/brevdev/microservices-demo.git"
// "git@github.com:brevdev/microservices-demo.git"
func clone(t *terminal.Terminal, url string, orgflag string, startStore StartStore) error {
	newWorkspace := MakeNewWorkspaceFromURL(url)

	var orgID string
	if orgflag == "" {
		activeorg, err := startStore.GetActiveOrganizationOrDefault()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if activeorg == nil {
			return fmt.Errorf("no org exist")
		}
		orgID = activeorg.ID
	} else {
		orgs, err := startStore.GetOrganizations(&store.GetOrganizationsOptions{Name: orgflag})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if len(orgs) == 0 {
			return fmt.Errorf("no org with name %s", orgflag)
		} else if len(orgs) > 1 {
			return fmt.Errorf("more than one org with name %s", orgflag)
		}
		orgID = orgs[0].ID
	}

	err := createWorkspace(t, newWorkspace, orgID, startStore)
	if err != nil {
		t.Vprint(t.Red(err.Error()))
	}
	return nil
}

type NewWorkspace struct {
	Name    string `json:"name"`
	GitRepo string `json:"gitRepo"`
}

func MakeNewWorkspaceFromURL(url string) NewWorkspace {
	if strings.Contains(url, "http") {
		split := strings.Split(url, ".com/")
		provider := strings.Split(split[0], "://")[1]

		if strings.Contains(split[1], ".git") {
			return NewWorkspace{
				GitRepo: fmt.Sprintf("%s.com:%s", provider, split[1]),
				Name:    strings.Split(split[1], ".git")[0],
			}
		} else {
			return NewWorkspace{
				GitRepo: fmt.Sprintf("%s.com:%s.git", provider, split[1]),
				Name:    split[1],
			}
		}
	} else {
		split := strings.Split(url, ".com:")
		provider := strings.Split(split[0], "@")[1]
		return NewWorkspace{
			GitRepo: fmt.Sprintf("%s.com:%s", provider, split[1]),
			Name:    strings.Split(split[1], ".git")[0],
		}
	}
}

func createWorkspace(t *terminal.Terminal, workspace NewWorkspace, orgID string, startStore StartStore) error {
	t.Vprint("\nWorkspace is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	options := store.NewCreateWorkspacesOptions(clusterID, workspace.Name).WithGitRepo(workspace.GitRepo)

	w, err := startStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("You can safely ctrl+c to exit\n")

	err = pollUntil(t, w.ID, "RUNNING", startStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour workspace is ready!"))
	t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.DNS))

	return nil
}

func pollUntil(t *terminal.Terminal, wsid string, state string, startStore StartStore) error {
	s := t.NewSpinner()
	isReady := false
	for !isReady {
		time.Sleep(5 * time.Second)
		s.Start()
		ws, err := startStore.GetWorkspace(wsid)
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
