// Package start is for starting Brev workspaces
package start

import (
	"fmt"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/files"
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
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	GetSetupScriptContentsByURL(url string) (string, error)
}

func NewCmdStart(t *terminal.Terminal, loginStartStore StartStore, noLoginStartStore StartStore) *cobra.Command {
	var org string
	var name string
	var detached bool
	var empty bool
	var is4x16 bool
	var workspaceClass string
	var setupScript string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "start",
		DisableFlagsInUseLine: true,
		Short:                 "Start a workspace if it's stopped, or create one from url",
		Long:                  startLong,
		Example:               startExample,
		// Args:                  cobra.ExactArgs(1),
		ValidArgsFunction: completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 && !empty {
				t.Vprintf(t.Red("An argument is required, or use the '--empty' flag\n"))
				return
			}

			if empty {
				err := createEmptyWorkspace(t, org, loginStartStore, name, detached, setupScript, workspaceClass)
				if err != nil {
					t.Vprintf(t.Red(err.Error()))
					return
				}
			} else {
				isURL := false
				if strings.Contains(args[0], "https://") || strings.Contains(args[0], "git@") {
					isURL = true
				}

				if isURL {
					// CREATE A WORKSPACE
					err := clone(t, args[0], org, loginStartStore, name, is4x16, setupScript, workspaceClass)
					if err != nil {
						t.Vprint(t.Red(err.Error()))
					}
				} else {
					// Start an existing one (either theirs or someone elses)
					err := startWorkspace(args[0], loginStartStore, t, detached, name, workspaceClass)
					if err != nil {
						t.Vprint(t.Red(err.Error()))
					}
				}
			}
		},
	}
	cmd.Flags().BoolVarP(&detached, "detached", "d", false, "run the command in the background instead of blocking the shell")
	cmd.Flags().BoolVarP(&empty, "empty", "e", false, "create an empty workspace")
	cmd.Flags().StringVarP(&name, "name", "n", "", "name your workspace when creating a new one")
	cmd.Flags().StringVarP(&workspaceClass, "class", "c", "", "workspace resource class (cpu x memory) default 2x8 [2x8, 4x16, 8x32, 16x32]")
	cmd.Flags().StringVarP(&setupScript, "setup-script", "s", "", "replace the default setup script")
	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org if creating a workspace)")
	err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noLoginStartStore, t))
	if err != nil {
		t.Errprint(err, "cli err")
	}
	return cmd
}

func createEmptyWorkspace(t *terminal.Terminal, orgflag string, startStore StartStore, name string, detached bool, setupScript string, workspaceClass string) error {
	// ensure name
	if len(name) == 0 {
		return fmt.Errorf("name field is required for empty workspaces")
	}

	// ensure org
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

	var setupScriptContents string
	var err error
	lines := files.GetAllAliases()
	if len(lines) > 0 {
		snip := files.GenerateSetupScript(lines)
		setupScriptContents += snip
	}
	if len(setupScript) > 0 {
		contents, err := startStore.GetSetupScriptContentsByURL(setupScript)
		setupScriptContents += "\n" + contents

		if err != nil {
			t.Vprintf(t.Red("Couldn't fetch setup script from %s\n", setupScript) + t.Yellow("Continuing with default setup script 👍"))
			return breverrors.WrapAndTrace(err)
		}
	}

	clusterID := config.GlobalConfig.GetDefaultClusterID()
	options := store.NewCreateWorkspacesOptions(clusterID, name)

	if workspaceClass != "" {
		options.WithClassID(workspaceClass)
	}

	user, err := startStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	options = resolveWorkspaceUserOptions(options, user)

	if len(setupScriptContents) > 0 {
		options.WithStartupScript(setupScriptContents)
	}

	w, err := startStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprint("\nWorkspace is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))

	if detached {
		return nil
	} else {
		err = pollUntil(t, w.ID, "RUNNING", startStore, true)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		t.Vprint(t.Green("\nYour workspace is ready!"))
		t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.GetLocalIdentifier(nil)))

		return nil
	}
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

func startWorkspace(workspaceName string, startStore StartStore, t *terminal.Terminal, detached bool, name string, workspaceClass string) error {
	workspace, err := getWorkspaceFromNameOrID(workspaceName, startStore)
	org, othererr := startStore.GetActiveOrganizationOrDefault()
	if othererr != nil {
		return breverrors.WrapAndTrace(othererr)
	}
	user, usererr := startStore.GetCurrentUser()
	if usererr != nil {
		return breverrors.WrapAndTrace(err)
	}
	if err != nil {
		// This is not an error yet-- the user might be trying to join a team's workspace
		if org == nil {
			return fmt.Errorf("no orgs exist")
		}
		workspaces, othererr := startStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{
			Name: workspaceName,
		})
		if othererr != nil {
			return breverrors.WrapAndTrace(othererr)
		}
		if len(workspaces) == 0 {
			return fmt.Errorf("your team has no projects named %s", workspaceName)
		}
		othererr = joinProjectWithNewWorkspace(workspaces[0], t, org.ID, startStore, name, user, workspaceClass)
		if othererr != nil {
			return breverrors.WrapAndTrace(othererr)
		}

	} else {
		if workspace.Status == "RUNNING" {
			t.Vprint(t.Yellow("Workspace is already running"))
			return nil
		}
		// TODO: check the workspace isn't running first!!!
		if workspaceClass != "" {
			t.Vprint(t.Yellow("Workspace already exists. Can not pass workspace class flag to start stopped workspace"))
			return nil
		}

		if len(name) > 0 {
			t.Vprint("Existing workspace found. Name flag ignored.")
		}

		startedWorkspace, err := startStore.StartWorkspace(workspace.ID)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		t.Vprintf(t.Yellow("\nWorkspace %s is starting. \nNote: this can take about a minute. Run 'brev ls' to check status\n\n", startedWorkspace.Name))

		// Don't poll and block the shell if detached flag is set
		if detached {
			return nil
		}

		err = pollUntil(t, workspace.ID, "RUNNING", startStore, true)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		t.Vprint(t.Green("\nYour workspace is ready!"))

		workspaces, err := startStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{UserID: user.ID})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		t.Vprintf(t.Green("\n\nConnect to your machine with:") +
			t.Yellow("\n\tssh %s\n", startedWorkspace.GetLocalIdentifier(workspaces)))
	}

	return nil
}

// "https://github.com/brevdev/microservices-demo.git
// "https://github.com/brevdev/microservices-demo.git"
// "git@github.com:brevdev/microservices-demo.git"
func joinProjectWithNewWorkspace(templateWorkspace entity.Workspace, t *terminal.Terminal, orgID string, startStore StartStore, name string, user *entity.User, workspaceClass string) error {
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	if workspaceClass == "" {
		workspaceClass = templateWorkspace.WorkspaceClassID
	}

	options := store.NewCreateWorkspacesOptions(clusterID, templateWorkspace.Name).WithGitRepo(templateWorkspace.GitRepo).WithWorkspaceClassID(workspaceClass)
	if len(name) > 0 {
		options.Name = name
	} else {
		t.Vprintf("Name flag omitted, using auto generated name: %s", t.Green(options.Name))
	}

	options = resolveWorkspaceUserOptions(options, user)

	t.Vprint("\nWorkspace is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))

	w, err := startStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, "RUNNING", startStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspaces, err := startStore.GetWorkspaces(orgID, &store.GetWorkspacesOptions{UserID: user.ID})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("\nConnect to your machine with:\n\tssh %s\n", w.GetLocalIdentifier(workspaces))

	return nil
}

func clone(t *terminal.Terminal, url string, orgflag string, startStore StartStore, name string, is4x16 bool, setupScript string, workspaceClass string) error {
	t.Vprintf("This is the setup script: %s", setupScript)
	// https://gist.githubusercontent.com/naderkhalil/4a45d4d293dc3a9eb330adcd5440e148/raw/3ab4889803080c3be94a7d141c7f53e286e81592/setup.sh
	// fetch contents of file
	// todo: read contents of file
	if is4x16 {
		workspaceClass = "4x16"
	}

	var setupScriptContents string
	var err error
	lines := files.GetAllAliases()
	if len(lines) > 0 {
		snip := files.GenerateSetupScript(lines)
		setupScriptContents += snip
	}
	if len(setupScript) > 0 {
		contents, err := startStore.GetSetupScriptContentsByURL(setupScript)
		setupScriptContents += "\n" + contents
		if err != nil {
			t.Vprintf(t.Red("Couldn't fetch setup script from %s\n", setupScript) + t.Yellow("Continuing with default setup script 👍"))
			return breverrors.WrapAndTrace(err)
		}
	}

	newWorkspace := MakeNewWorkspaceFromURL(url)

	if len(name) > 0 {
		newWorkspace.Name = name
	} else {
		t.Vprintf("Name flag omitted, using auto generated name: %s", t.Green(newWorkspace.Name))
	}

	var orgID string
	if orgflag == "" {
		activeorg, err2 := startStore.GetActiveOrganizationOrDefault()
		if err2 != nil {
			return breverrors.WrapAndTrace(err)
		}
		if activeorg == nil {
			return fmt.Errorf("no org exist")
		}
		orgID = activeorg.ID
	} else {
		orgs, err2 := startStore.GetOrganizations(&store.GetOrganizationsOptions{Name: orgflag})
		if err2 != nil {
			return breverrors.WrapAndTrace(err)
		}
		if len(orgs) == 0 {
			return fmt.Errorf("no org with name %s", orgflag)
		} else if len(orgs) > 1 {
			return fmt.Errorf("more than one org with name %s", orgflag)
		}
		orgID = orgs[0].ID
	}

	err = createWorkspace(t, newWorkspace, orgID, startStore, workspaceClass, setupScriptContents)
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
	var name string
	if strings.Contains(url, "http") {
		split := strings.Split(url, ".com/")
		provider := strings.Split(split[0], "://")[1]

		if strings.Contains(split[1], ".git") {
			name = strings.Split(split[1], ".git")[0]
			if strings.Contains(name, "/") {
				name = strings.Split(name, "/")[1]
			}
			return NewWorkspace{
				GitRepo: fmt.Sprintf("%s.com:%s", provider, split[1]),
				Name:    name,
			}
		} else {
			name = split[1]
			if strings.Contains(name, "/") {
				name = strings.Split(name, "/")[1]
			}
			return NewWorkspace{
				GitRepo: fmt.Sprintf("%s.com:%s.git", provider, split[1]),
				Name:    name,
			}
		}
	} else {
		split := strings.Split(url, ".com:")
		provider := strings.Split(split[0], "@")[1]
		name = strings.Split(split[1], ".git")[0]
		if strings.Contains(name, "/") {
			name = strings.Split(name, "/")[1]
		}
		return NewWorkspace{
			GitRepo: fmt.Sprintf("%s.com:%s", provider, split[1]),
			Name:    name,
		}
	}
}

func createWorkspace(t *terminal.Terminal, workspace NewWorkspace, orgID string, startStore StartStore, workspaceClass string, setupScript string) error {
	t.Vprint("\nWorkspace is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	options := store.NewCreateWorkspacesOptions(clusterID, workspace.Name).WithGitRepo(workspace.GitRepo)

	user, err := startStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if workspaceClass != "" {
		options = options.WithWorkspaceClassID(workspaceClass)
	}

	options = resolveWorkspaceUserOptions(options, user)

	if len(setupScript) > 0 {
		options.WithStartupScript(setupScript)
	}

	w, err := startStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, "RUNNING", startStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour workspace is ready!"))
	t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.GetLocalIdentifier(nil)))

	return nil
}

func pollUntil(t *terminal.Terminal, wsid string, state string, startStore StartStore, canSafelyExit bool) error { //nolint: unparam // want to take state as a variable
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
		s.Suffix = "  workspace is " + strings.ToLower(ws.Status)
		if ws.Status == state {
			s.Suffix = "Workspace is ready!"
			s.Stop()
			isReady = true
		}
	}
	return nil
}

// NOTE: this function is copy/pasted in many places. If you modify it, modify it elsewhere.
// Reasoning: there wasn't a utils file so I didn't know where to put it
//                + not sure how to pass a generic "store" object
func getWorkspaceFromNameOrID(nameOrID string, sstore StartStore) (*entity.WorkspaceWithMeta, error) {
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

// NADER IS SO FUCKING SORRY FOR DOING THIS TWICE BUT I HAVE NO CLUE WHERE THIS HELPER FUNCTION SHOULD GO SO ITS COPY/PASTED ELSEWHERE
// IF YOU MODIFY IT MODIFY IT EVERYWHERE OR PLEASE PUT IT IN ITS PROPER PLACE. thank you you're the best <3
func WorkspacesFromWorkspaceWithMeta(wwm []entity.WorkspaceWithMeta) []entity.Workspace {
	var workspaces []entity.Workspace

	for _, v := range wwm {
		workspaces = append(workspaces, entity.Workspace{
			ID:                v.ID,
			Name:              v.Name,
			WorkspaceGroupID:  v.WorkspaceGroupID,
			OrganizationID:    v.OrganizationID,
			WorkspaceClassID:  v.WorkspaceClassID,
			CreatedByUserID:   v.CreatedByUserID,
			DNS:               v.DNS,
			Status:            v.Status,
			Password:          v.Password,
			GitRepo:           v.GitRepo,
			Version:           v.Version,
			WorkspaceTemplate: v.WorkspaceTemplate,
		})
	}

	return workspaces
}
