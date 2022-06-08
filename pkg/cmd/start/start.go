// Package start is for starting Brev workspaces
package start

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/mergeshells" //nolint:typecheck // uses generic code
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
	util.GetWorkspaceByNameOrIDErrStore
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	StartWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	GetSetupScriptContentsByURL(url string) (string, error)
	GetFileAsString(path string) (string, error)
}

func NewCmdStart(t *terminal.Terminal, startStore StartStore, noLoginStartStore StartStore) *cobra.Command {
	var org string
	var name string
	var detached bool
	var empty bool
	var workspaceClass string
	var setupScript string
	var setupRepo string
	var setupPath string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "start",
		DisableFlagsInUseLine: true,
		Short:                 "Start a workspace if it's stopped, or create one from url",
		Long:                  startLong,
		Example:               startExample,
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := ""
			if len(args) > 0 {
				repo = args[0]
			}
			err := runStartWorkspace(t, StartOptions{
				Repo:           repo,
				Name:           name,
				OrgName:        org,
				SetupScript:    setupScript,
				SetupRepo:      setupRepo,
				SetupPath:      setupPath,
				WorkspaceClass: workspaceClass,
				Detached:       detached,
			}, startStore)
			if err != nil {
				if strings.Contains(err.Error(), "duplicate workspace with name") {
					t.Vprint(t.Yellow("try running:"))
					t.Vprint(t.Yellow("\tbrev start --name [different name] [repo] # or"))
					t.Vprint(t.Yellow("\tbrev delete [name]"))
				}
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&detached, "detached", "d", false, "run the command in the background instead of blocking the shell")
	cmd.Flags().BoolVarP(&empty, "empty", "e", false, "create an empty workspace")
	cmd.Flags().StringVarP(&name, "name", "n", "", "name your workspace when creating a new one")
	cmd.Flags().StringVarP(&workspaceClass, "class", "c", "", "workspace resource class (cpu x memory) default 2x8 [2x8, 4x16, 8x32, 16x32]")
	cmd.Flags().StringVarP(&setupScript, "setup-script", "s", "", "replace the default setup script")
	cmd.Flags().StringVarP(&setupRepo, "setup-repo", "r", "", "use a setup script in a custom repository")
	cmd.Flags().StringVarP(&setupPath, "setup-path", "p", "", "path to setup script in custom repository")
	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org if creating a workspace)")
	err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noLoginStartStore, t))
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(err)
		t.Errprint(err, "cli err")
	}
	return cmd
}

type StartOptions struct {
	Repo           string
	Name           string
	OrgName        string
	SetupScript    string
	SetupRepo      string
	SetupPath      string
	WorkspaceClass string
	Detached       bool
}

func runStartWorkspace(t *terminal.Terminal, options StartOptions, startStore StartStore) error {
	if options.Repo == "" { // empty
		if options.Name == "" {
			return breverrors.NewValidationError("must provide a --name")
		}
		err := createEmptyWorkspace(t, options, startStore)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		isURL := false
		if strings.Contains(options.Repo, "https://") || strings.Contains(options.Repo, "git@") {
			isURL = true
		}

		if isURL {
			// CREATE A WORKSPACE
			err := clone(t, options.SetupScript, options, startStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
		} else {
			workspace, _ := util.GetUserWorkspaceByNameOrIDErr(startStore, options.Repo) // ignoring err todo handle me better
			if workspace == nil {
				// get org, check for workspace to join before assuming start via path
				activeOrg, err := startStore.GetActiveOrganizationOrDefault()
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
				user, err := startStore.GetCurrentUser()
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
				workspaces, err := startStore.GetWorkspaces(activeOrg.ID, &store.GetWorkspacesOptions{
					Name: options.Repo,
				})
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
				if len(workspaces) == 0 {
					// then this is a path, and we should import dependencies from it and start
					err = startWorkspaceFromPath(t, options, startStore)
					if err != nil {
						return breverrors.WrapAndTrace(err)
					}
				} else {
					// the user wants to join a workspace
					err = joinProjectWithNewWorkspace(t, workspaces[0], activeOrg.ID, startStore, user, options)
					if err != nil {
						return breverrors.WrapAndTrace(err)
					}
				}

			} else {
				// Start an existing one (either theirs or someone elses)
				err := startWorkspace(startStore, t, options)
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
			}
		}
	}
	return nil
}

func startWorkspaceFromPath(t *terminal.Terminal, options StartOptions, startStore StartStore) error {
	pathExists := dirExists(options.Repo)
	if !pathExists {
		return fmt.Errorf(strings.Join([]string{"Path:", options.Repo, "does not exist."}, " "))
	}

	var gitpath string
	if options.Repo == "." {
		gitpath = filepath.Join(".git", "config")
	} else {
		gitpath = filepath.Join(options.Repo, ".git", "config")
	}
	file, error := startStore.GetFileAsString(gitpath)
	if error != nil {
		return fmt.Errorf(strings.Join([]string{"Could not read .git/config at", options.Repo}, " "))
	}
	// Get GitUrl
	var gitURL string
	for _, v := range strings.Split(file, "\n") {
		if strings.Contains(v, "url") {
			gitURL = strings.Split(v, "= ")[1]
		}
	}
	if len(gitURL) == 0 {
		return fmt.Errorf("no git url found")
	}
	gitParts := strings.Split(gitURL, "/")
	options.Name = strings.Split(gitParts[len(gitParts)-1], ".")[0]
	localSetupPath := filepath.Join(options.Repo, ".brev", "setup.sh")
	if options.Repo == "." {
		localSetupPath = filepath.Join(".brev", "setup.sh")
	}
	if !dirExists(localSetupPath) {
		fmt.Println(strings.Join([]string{"Generating setup script at", localSetupPath}, "\n"))
		mergeshells.ImportPath(t, options.Repo, startStore)
		fmt.Println("setup script generated.")
	}

	err := clone(t, localSetupPath, options, startStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return err
}

// exists returns whether the given file or directory exists
func dirExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func createEmptyWorkspace(t *terminal.Terminal, options StartOptions, startStore StartStore) error {
	// ensure name
	if len(options.Name) == 0 {
		return breverrors.NewValidationError("name field is required for empty workspaces")
	}

	// ensure org
	var orgID string
	if options.OrgName == "" {
		activeorg, err := startStore.GetActiveOrganizationOrDefault()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if activeorg == nil {
			return breverrors.NewValidationError("no org exist")
		}
		orgID = activeorg.ID
	} else {
		orgs, err := startStore.GetOrganizations(&store.GetOrganizationsOptions{Name: options.OrgName})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if len(orgs) == 0 {
			return breverrors.NewValidationError(fmt.Sprintf("no org with name %s", options.OrgName))
		} else if len(orgs) > 1 {
			return breverrors.NewValidationError(fmt.Sprintf("more than one org with name %s", options.OrgName))
		}
		orgID = orgs[0].ID
	}

	var setupScriptContents string
	var err error
	if len(options.SetupScript) > 0 {
		contents, err1 := startStore.GetSetupScriptContentsByURL(options.SetupScript)
		setupScriptContents += "\n" + contents

		if err1 != nil {
			t.Vprintf(t.Red("Couldn't fetch setup script from %s\n", options.SetupScript) + t.Yellow("Continuing with default setup script 👍"))
			return breverrors.WrapAndTrace(err1)
		}
	}

	clusterID := config.GlobalConfig.GetDefaultClusterID()
	cwOptions := store.NewCreateWorkspacesOptions(clusterID, options.Name)

	if options.WorkspaceClass != "" {
		cwOptions.WithClassID(options.WorkspaceClass)
	}

	user, err := startStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	cwOptions = resolveWorkspaceUserOptions(cwOptions, user)

	if len(setupScriptContents) > 0 {
		cwOptions.WithStartupScript(setupScriptContents)
	}

	w, err := startStore.CreateWorkspace(orgID, cwOptions)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprint("\nWorkspace is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))

	if options.Detached {
		return nil
	} else {
		err = pollUntil(t, w.ID, "RUNNING", startStore, true)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		fmt.Print("\n")
		t.Vprint(t.Green("Your workspace is ready!\n"))
		displayConnectBreadCrumb(t, w)

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

func startWorkspace(startStore StartStore, t *terminal.Terminal, startOptions StartOptions) error {
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(startStore, startOptions.Name)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
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
			return breverrors.NewValidationError("no orgs exist")
		}
		workspaces, othererr := startStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{
			Name: startOptions.Name,
		})
		if othererr != nil {
			return breverrors.WrapAndTrace(othererr)
		}
		if len(workspaces) == 0 {
			return breverrors.NewValidationError(fmt.Sprintf("your team has no projects named %s", startOptions.Name))
		}
		othererr = joinProjectWithNewWorkspace(t, workspaces[0], org.ID, startStore, user, startOptions)
		if othererr != nil {
			return breverrors.WrapAndTrace(othererr)
		}

	} else {
		if workspace.Status == "RUNNING" {
			t.Vprint(t.Yellow("Workspace is already running"))
			return nil
		}
		if startOptions.WorkspaceClass != "" {
			return breverrors.NewValidationError("Workspace already exists. Can not pass workspace class flag to start stopped workspace")
		}

		if startOptions.Name != "" {
			t.Vprint("Existing workspace found. Name flag ignored.")
		}

		startedWorkspace, err := startStore.StartWorkspace(workspace.ID)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		t.Vprintf(t.Yellow("\nWorkspace %s is starting. \nNote: this can take about a minute. Run 'brev ls' to check status\n\n", startedWorkspace.Name))

		// Don't poll and block the shell if detached flag is set
		if startOptions.Detached {
			return nil
		}

		err = pollUntil(t, workspace.ID, "RUNNING", startStore, true)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		fmt.Print("\n")
		t.Vprint(t.Green("Your workspace is ready!\n"))
		displayConnectBreadCrumb(t, startedWorkspace)
	}

	return nil
}

// "https://github.com/brevdev/microservices-demo.git
// "https://github.com/brevdev/microservices-demo.git"
// "git@github.com:brevdev/microservices-demo.git"
func joinProjectWithNewWorkspace(t *terminal.Terminal, templateWorkspace entity.Workspace, orgID string, startStore StartStore, user *entity.User, startOptions StartOptions) error {
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	if startOptions.WorkspaceClass == "" {
		startOptions.WorkspaceClass = templateWorkspace.WorkspaceClassID
	}

	cwOptions := store.NewCreateWorkspacesOptions(clusterID, templateWorkspace.Name).WithGitRepo(templateWorkspace.GitRepo).WithWorkspaceClassID(startOptions.WorkspaceClass)
	if startOptions.Name != "" {
		cwOptions.Name = startOptions.Name
	} else {
		t.Vprintf("Name flag omitted, using auto generated name: %s", t.Green(cwOptions.Name))
	}

	cwOptions = resolveWorkspaceUserOptions(cwOptions, user)

	t.Vprint("\nWorkspace is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))

	w, err := startStore.CreateWorkspace(orgID, cwOptions)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, "RUNNING", startStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	displayConnectBreadCrumb(t, w)

	return nil
}

func IsUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func clone(t *terminal.Terminal, setupScriptURLOrPath string, startOptions StartOptions, startStore StartStore) error {
	t.Vprintf("This is the setup script: %s", setupScriptURLOrPath)
	// https://gist.githubusercontent.com/naderkhalil/4a45d4d293dc3a9eb330adcd5440e148/raw/3ab4889803080c3be94a7d141c7f53e286e81592/setup.sh
	// fetch contents of file
	// todo: read contents of file

	var setupScriptContents string
	var err error
	// TODO: this makes for really good DX, but should be added as a personal setting on the User model
	// lines := files.GetAllAliases()
	// if len(lines) > 0 {
	// 	snip := files.GenerateSetupScript(lines)
	// 	setupScriptContents += snip
	// }
	if len(startOptions.Repo) > 0 && len(startOptions.SetupPath) > 0 {
		// STUFF HERE
	} else if len(setupScriptURLOrPath) > 0 {
		if IsUrl(setupScriptURLOrPath) {
			contents, err1 := startStore.GetSetupScriptContentsByURL(setupScriptURLOrPath)
			if err1 != nil {
				t.Vprintf(t.Red("Couldn't fetch setup script from %s\n", setupScriptURLOrPath) + t.Yellow("Continuing with default setup script 👍"))
				return breverrors.WrapAndTrace(err1)
			}
			setupScriptContents += "\n" + contents
		} else {
			// ERROR: not sure what this use case is for
			var err2 error
			setupScriptContents, err2 = startStore.GetFileAsString(setupScriptURLOrPath)
			if err2 != nil {
				return breverrors.WrapAndTrace(err2)
			}
		}
	}

	newWorkspace := MakeNewWorkspaceFromURL(startOptions.Repo)

	if (startOptions.Name) != "" {
		newWorkspace.Name = startOptions.Name
	} else {
		t.Vprintf("Name flag omitted, using auto generated name: %s", t.Green(newWorkspace.Name))
	}

	var orgID string
	if startOptions.OrgName == "" {
		activeorg, err2 := startStore.GetActiveOrganizationOrDefault()
		if err2 != nil {
			return breverrors.WrapAndTrace(err)
		}
		if activeorg == nil {
			return breverrors.NewValidationError("no org exist")
		}
		orgID = activeorg.ID
	} else {
		orgs, err2 := startStore.GetOrganizations(&store.GetOrganizationsOptions{Name: startOptions.OrgName})
		if err2 != nil {
			return breverrors.WrapAndTrace(err)
		}
		if len(orgs) == 0 {
			return breverrors.NewValidationError(fmt.Sprintf("no org with name %s", startOptions.OrgName))
		} else if len(orgs) > 1 {
			return breverrors.NewValidationError(fmt.Sprintf("more than one org with name %s", startOptions.OrgName))
		}
		orgID = orgs[0].ID
	}

	err = createWorkspace(t, newWorkspace, orgID, startStore, startOptions)
	if err != nil {
		return breverrors.WrapAndTrace(err)
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

func createWorkspace(t *terminal.Terminal, workspace NewWorkspace, orgID string, startStore StartStore, startOptions StartOptions) error {
	t.Vprint("\nWorkspace is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))
	clusterID := config.GlobalConfig.GetDefaultClusterID()

	options := store.NewCreateWorkspacesOptions(clusterID, workspace.Name).WithGitRepo(workspace.GitRepo)

	user, err := startStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if startOptions.WorkspaceClass != "" {
		options = options.WithWorkspaceClassID(startOptions.WorkspaceClass)
	}

	options = resolveWorkspaceUserOptions(options, user)

	if startOptions.SetupRepo != "" {
		options.WithCustomSetupRepo(startOptions.SetupRepo, startOptions.SetupPath)
	} else if startOptions.SetupPath != "" {
		options.StartupScriptPath = startOptions.SetupPath
	}

	if startOptions.SetupScript != "" {
		options.WithStartupScript(startOptions.SetupScript)
	}

	w, err := startStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = pollUntil(t, w.ID, "RUNNING", startStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Print("\n")
	t.Vprint(t.Green("Your workspace is ready!\n"))

	displayConnectBreadCrumb(t, w)

	return nil
}

func displayConnectBreadCrumb(t *terminal.Terminal, workspace *entity.Workspace) {
	t.Vprintf(t.Green("Connect to the workspace:\n"))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev open %s\t# brev open <NAME> -> open workspace in preferred editor\n", workspace.Name)))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev shell %s\t# brev shell <NAME> -> shell into workspace\n", workspace.Name)))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tssh %s\t# ssh <SSH-NAME> -> ssh directly to workspace\n", workspace.GetLocalIdentifier())))
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
		s.Suffix = "  workspace is " + strings.ToLower(ws.Status)
		if ws.Status == state {
			s.Suffix = "Workspace is ready!"
			s.Stop()
			isReady = true
		}
	}
	return nil
}
