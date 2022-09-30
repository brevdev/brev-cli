// Package start is for starting Brev workspaces
package start

import (
	"fmt"
	"net/url"
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
	allutil "github.com/brevdev/brev-cli/pkg/util"
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
	instanceTypes = []string{"p4d.24xlarge", "p3.2xlarge", "p3.8xlarge", "p3.16xlarge", "p3dn.24xlarge", "p2.xlarge", "p2.8xlarge", "p2.16xlarge", "g5.xlarge", "g5.2xlarge", "g5.4xlarge", "g5.8xlarge", "g5.16xlarge", "g5.12xlarge", "g5.24xlarge", "g5.48xlarge", "g5g.xlarge", "g5g.2xlarge", "g5g.4xlarge", "g5g.8xlarge", "g5g.16xlarge", "g5g.metal", "g4dn.xlarge", "g4dn.2xlarge", "g4dn.4xlarge", "g4dn.8xlarge", "g4dn.16xlarge", "g4dn.12xlarge", "g4dn.metal", "g4ad.xlarge", "g4ad.2xlarge", "g4ad.4xlarge", "g4ad.8xlarge", "g4ad.16xlarge", "g3s.xlarge", "g3.4xlarge", "g3.8xlarge", "g3.16xlarge"}
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

func validateInstanceType(instanceType string) bool {
	for _, v := range instanceTypes {
		if instanceType == v {
			return true
		}
	}
	return false
}

func NewCmdStart(t *terminal.Terminal, startStore StartStore, noLoginStartStore StartStore) *cobra.Command {
	var org string
	var name string
	var detached bool
	var empty bool
	var setupScript string
	var setupRepo string
	var setupPath string
	var gpu string
	var cpu string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "start",
		DisableFlagsInUseLine: true,
		Short:                 "Start a dev environment if it's stopped, or create one from url",
		Long:                  startLong,
		Example:               startExample,
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoOrPathOrNameOrID := ""
			if len(args) > 0 {
				repoOrPathOrNameOrID = args[0]
			}

			if gpu != "" {
				isValid := validateInstanceType(gpu)
				if !isValid {
					err := fmt.Errorf("invalid GPU instance type: %s", gpu)
					return breverrors.WrapAndTrace(err)
				}
			}

			err := runStartWorkspace(t, StartOptions{
				RepoOrPathOrNameOrID: repoOrPathOrNameOrID,
				Name:                 name,
				OrgName:              org,
				SetupScript:          setupScript,
				SetupRepo:            setupRepo,
				SetupPath:            setupPath,
				WorkspaceClass:       cpu,
				Detached:             detached,
				InstanceType:         gpu,
			}, startStore)
			if err != nil {
				if strings.Contains(err.Error(), "duplicate environment with name") {
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
	cmd.Flags().StringVarP(&cpu, "cpu", "c", "", "CPU instance type. Defaults to 2x8 [2x8, 4x16, 8x32, 16x32]. See docs.brev.dev/cpu for details")
	cmd.Flags().StringVarP(&setupScript, "setup-script", "s", "", "takes a raw gist url to an env setup script")
	cmd.Flags().StringVarP(&setupRepo, "setup-repo", "r", "", "repo that holds env setup script. you must pass in --setup-path if you use this argument")
	cmd.Flags().StringVarP(&setupPath, "setup-path", "p", "", "path to env setup script. If you include --setup-repo we will apply this argument to that repo")
	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org if creating a workspace)")
	// GPU options
	cmd.Flags().StringVarP(&gpu, "gpu", "g", "", "GPU instance type. See docs.brev.dev/gpu for details")
	err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noLoginStartStore, t))
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err))
		fmt.Print(breverrors.WrapAndTrace(err))
	}
	return cmd
}

type StartOptions struct {
	RepoOrPathOrNameOrID string // todo make invidual options
	Name                 string
	OrgName              string
	SetupScript          string
	SetupRepo            string
	SetupPath            string
	WorkspaceClass       string
	Detached             bool
	InstanceType         string
}

func runStartWorkspace(t *terminal.Terminal, options StartOptions, startStore StartStore) error {
	user, err := startStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	didStart, err := maybeStartEmpty(t, user, options, startStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if didStart {
		return nil
	}

	didStart, err = maybeStartFromGitURL(t, user, options, startStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if didStart {
		return nil
	}

	didStart, err = maybeStartStoppedOrJoin(t, user, options, startStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if didStart {
		return nil
	}

	didStart, err = maybeStartWithLocalPath(options, user, t, startStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if didStart {
		return nil
	}

	return nil
}

func maybeStartWithLocalPath(options StartOptions, user *entity.User, t *terminal.Terminal, startStore StartStore) (bool, error) {
	if allutil.DoesPathExist(options.RepoOrPathOrNameOrID) {
		err := startWorkspaceFromPath(user, t, options, startStore)
		if err != nil {
			return false, breverrors.WrapAndTrace(err)
		}
		return true, nil
	} else {
		t.Print(t.Yellow("tried to start with local path but path not found"))
	}
	return false, nil
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
			breverrors.NewValidationError(fmt.Sprintf("multiple dev environments found with id/name %s", options.RepoOrPathOrNameOrID))
		}
		if allutil.DoesPathExist(options.RepoOrPathOrNameOrID) {
			t.Print(t.Yellow(fmt.Sprintf("Warning: local path found and dev environment name/id found %s. Using dev environment name/id. If you meant to specify a local path change directory and try again.", options.RepoOrPathOrNameOrID)))
		}
		err := startStopppedWorkspace(&userWorkspaces[0], startStore, t, options)
		if err != nil {
			return false, breverrors.WrapAndTrace(err)
		}
		return true, nil
	}

	if len(workspaces) > 0 {
		err = joinProjectWithNewWorkspace(t, workspaces[0], org.ID, startStore, user, options)
		if err != nil {
			return false, breverrors.WrapAndTrace(err)
		}
		return true, nil
	}
	return false, nil
}

func maybeStartFromGitURL(t *terminal.Terminal, user *entity.User, options StartOptions, startStore StartStore) (bool, error) {
	if allutil.IsGitURL(options.RepoOrPathOrNameOrID) { // todo this is function is not complete, some cloneable urls are not identified
		err := createNewWorkspaceFromGit(user, t, options.SetupScript, options, startStore)
		if err != nil {
			return true, breverrors.WrapAndTrace(err)
		}
		return true, nil
	}
	return false, nil
}

func maybeStartEmpty(t *terminal.Terminal, user *entity.User, options StartOptions, startStore StartStore) (bool, error) {
	if options.RepoOrPathOrNameOrID == "" {
		err := createEmptyWorkspace(user, t, options, startStore)
		if err != nil {
			return true, breverrors.WrapAndTrace(err)
		}
		return true, nil
	}
	return false, nil
}

func startWorkspaceFromPath(user *entity.User, t *terminal.Terminal, options StartOptions, startStore StartStore) error {
	pathExists := allutil.DoesPathExist(options.RepoOrPathOrNameOrID)
	if !pathExists {
		return fmt.Errorf(strings.Join([]string{"Path:", options.RepoOrPathOrNameOrID, "does not exist."}, " "))
	}
	var gitpath string
	if options.RepoOrPathOrNameOrID == "." {
		gitpath = filepath.Join(".git", "config")
	} else {
		gitpath = filepath.Join(options.RepoOrPathOrNameOrID, ".git", "config")
	}
	file, error := startStore.GetFileAsString(gitpath)
	if error != nil {
		return fmt.Errorf(strings.Join([]string{"Could not read .git/config at", options.RepoOrPathOrNameOrID}, " "))
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
	localSetupPath := filepath.Join(options.RepoOrPathOrNameOrID, ".brev", "setup.sh")
	if options.RepoOrPathOrNameOrID == "." {
		localSetupPath = filepath.Join(".brev", "setup.sh")
	}
	if !allutil.DoesPathExist(localSetupPath) {
		fmt.Println(strings.Join([]string{"Generating setup script at", localSetupPath}, "\n"))
		mergeshells.ImportPath(t, options.RepoOrPathOrNameOrID, startStore)
		fmt.Println("setup script generated.")
	}

	// createNewWorkspaceFromGit expects this field to be a git url, but above
	// logic wants it to be the directory path, so set it only before calling
	// createNewWorkspaceFromGit
	options.RepoOrPathOrNameOrID = gitURL
	err := createNewWorkspaceFromGit(user, t, localSetupPath, options, startStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return err
}

func createEmptyWorkspace(user *entity.User, t *terminal.Terminal, options StartOptions, startStore StartStore) error {
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

	cwOptions = resolveWorkspaceUserOptions(cwOptions, user)

	if len(setupScriptContents) > 0 {
		cwOptions.WithStartupScript(setupScriptContents)
	}

	if options.InstanceType != "" {
		cwOptions.WithInstanceType(options.InstanceType)
	}

	s := t.NewSpinner()
	s.Suffix = " Creating your instance. Hang tight 🤙"
	s.Start()
	w, err := startStore.CreateWorkspace(orgID, cwOptions)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	s.Stop()
	t.Vprint("Dev environment is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))

	if options.Detached {
		return nil
	} else {
		err = pollUntil(t, w.ID, entity.Running, startStore, true)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		fmt.Print("\n")
		t.Vprint(t.Green("Your dev environment is ready!\n"))
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

func startStopppedWorkspace(workspace *entity.Workspace, startStore StartStore, t *terminal.Terminal, startOptions StartOptions) error {
	if workspace.Status != entity.Stopped {
		return breverrors.NewValidationError(fmt.Sprintf("Dev environment is not stopped status=%s", workspace.Status))
	}
	if startOptions.WorkspaceClass != "" {
		return breverrors.NewValidationError("Dev environment already exists. Can not pass dev environment class flag to start stopped dev environment")
	}

	if startOptions.Name != "" {
		t.Vprint("Existing dev environment found. Name flag ignored.")
	}

	startedWorkspace, err := startStore.StartWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf(t.Yellow("Dev environment %s is starting. \nNote: this can take about a minute. Run 'brev ls' to check status\n\n", startedWorkspace.Name))

	// Don't poll and block the shell if detached flag is set
	if startOptions.Detached {
		return nil
	}

	err = pollUntil(t, workspace.ID, entity.Running, startStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Print("\n")
	t.Vprint(t.Green("Your dev environment is ready!\n"))
	displayConnectBreadCrumb(t, startedWorkspace)

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
		t.Vprintf("Name flag omitted, using auto generated name: %s\n", t.Green(cwOptions.Name))
	}

	cwOptions = resolveWorkspaceUserOptions(cwOptions, user)

	s := t.NewSpinner()
	s.Suffix = " Creating your instance. Hang tight 🤙"
	s.Start()
	w, err := startStore.CreateWorkspace(orgID, cwOptions)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	s.Stop()
	t.Vprint("Dev environment is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))

	err = pollUntil(t, w.ID, entity.Running, startStore, true)
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

func createNewWorkspaceFromGit(user *entity.User, t *terminal.Terminal, setupScriptURLOrPath string, startOptions StartOptions, startStore StartStore) error {
	// https://gist.githubusercontent.com/naderkhalil/4a45d4d293dc3a9eb330adcd5440e148/raw/3ab4889803080c3be94a7d141c7f53e286e81592/setup.sh
	// fetch contents of file
	// todo: read contents of file

	var setupScriptContents string
	var err error
	if len(startOptions.RepoOrPathOrNameOrID) > 0 && len(startOptions.SetupPath) > 0 {
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

	newWorkspace := MakeNewWorkspaceFromURL(startOptions.RepoOrPathOrNameOrID)

	if (startOptions.Name) != "" {
		newWorkspace.Name = startOptions.Name
	} else {
		t.Vprintf("Name flag omitted, using auto generated name: %s\n", t.Green(newWorkspace.Name))
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

	err = createWorkspace(user, t, newWorkspace, orgID, startStore, startOptions)
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

func createWorkspace(user *entity.User, t *terminal.Terminal, workspace NewWorkspace, orgID string, startStore StartStore, startOptions StartOptions) error {
	t.Vprint("Dev environment is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))
	clusterID := config.GlobalConfig.GetDefaultClusterID()

	options := store.NewCreateWorkspacesOptions(clusterID, workspace.Name).WithGitRepo(workspace.GitRepo)

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

	if startOptions.InstanceType != "" {
		options.WithInstanceType(startOptions.InstanceType)
	}

	s := t.NewSpinner()
	s.Suffix = " Creating your instance. Hang tight 🤙"
	s.Start()
	w, err := startStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	s.Stop()

	err = pollUntil(t, w.ID, entity.Running, startStore, true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Print("\n")
	t.Vprint(t.Green("Your dev environment is ready!\n"))

	displayConnectBreadCrumb(t, w)

	return nil
}

func displayConnectBreadCrumb(t *terminal.Terminal, workspace *entity.Workspace) {
	t.Vprintf(t.Green("Connect to the dev environment:\n"))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev open %s\t# brev open <NAME> -> open dev environment in preferred editor\n", workspace.Name)))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev shell %s\t# brev shell <NAME> -> ssh into dev environment (shortcut)\n", workspace.Name)))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tssh %s\t# ssh <SSH-NAME> -> ssh directly to dev environment\n", workspace.GetLocalIdentifier())))
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
		s.Suffix = "  environment is " + strings.ToLower(ws.Status)
		if ws.Status == state {
			s.Suffix = "Environment is ready!"
			s.Stop()
			isReady = true
		}
	}
	return nil
}
