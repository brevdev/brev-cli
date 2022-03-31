package importpkg

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/setupscript"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	"golang.org/x/text/encoding/charmap"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var (
	importLong    = "Import your current dev environment. Brev will try to fix errors along the way."
	importExample = `
  brev import <path>
  brev import .
  brev import ../my-broken-folder
	`
)

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

type ImportStore interface {
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	StartWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	GetDotGitConfigFile(path string) (string, error)
	GetDependenciesForImport(path string) (*store.Dependencies, error)
}

func NewCmdImport(t *terminal.Terminal, loginImportStore ImportStore, noLoginImportStore ImportStore) *cobra.Command {
	var org string
	var name string
	var detached bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "import",
		DisableFlagsInUseLine: true,
		Short:                 "Import your dev environment and have Brev try to fix errors along the way",
		Long:                  importLong,
		Example:               importExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginImportStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			err := startWorkspaceFromLocallyCloneRepo(t, org, loginImportStore, name, detached, args[0])
			if err != nil {
				t.Vprintf(t.Red(err.Error()))
				return
			}
		},
	}
	cmd.Flags().BoolVarP(&detached, "detached", "d", false, "run the command in the background instead of blocking the shell")
	cmd.Flags().StringVarP(&name, "name", "n", "", "name your workspace when creating a new one")
	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org if creating a workspace)")
	err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noLoginImportStore, t))
	if err != nil {
		t.Errprint(err, "cli err")
	}
	return cmd
}

func startWorkspaceFromLocallyCloneRepo(t *terminal.Terminal, orgflag string, importStore ImportStore, name string, detached bool, path string) error {
	pathExists := dirExists(path)
	if !pathExists {
		return breverrors.WrapAndTrace(errors.New("path provided does not exist"))
	}

	file, err := importStore.GetDotGitConfigFile(path)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Get GitUrl
	var gitURL string
	for _, v := range strings.Split(file, "\n") {
		if strings.Contains(v, "url") {
			gitURL = strings.Split(v, "= ")[1]
		}
	}
	if len(gitURL) == 0 {
		return breverrors.WrapAndTrace(errors.New("no git url found"))
	}

	node := isNode(t, path)
	if node != nil && len(*node)>0 {
		t.Vprintf("node-"+ *node + "---" + path)
	} else if node != nil && len(*node)==0  {
		t.Vprintf("node-our version" + "---" + path)
	}
	
	rust := isRust(t, path)
	if rust {
		t.Vprintf("rust" + "---" + path)
	}

	golang := getGoVersion(t, path)
	if golang=="" {
		t.Vprintf("golang our version" + "---" + path)
	} else {
		t.Vprintf("golang"+ golang + "---" + path)
	}

	
	fmt.Println("GitUrl: ", gitURL)


	// err = clone(t, gitURL, orgflag, importStore, name, deps, detached)
	// if err != nil {
	// 	return breverrors.WrapAndTrace(err)
	// }

	return nil
}

func clone(t *terminal.Terminal, url string, orgflag string, importStore ImportStore, name string, deps *store.Dependencies, detached bool) error {
	newWorkspace := MakeNewWorkspaceFromURL(url)

	if len(name) > 0 {
		newWorkspace.Name = name
	} else {
		t.Vprintf("Name flag omitted, using auto generated name: %s", t.Green(newWorkspace.Name))
	}

	var orgID string
	if orgflag == "" {
		activeorg, err := importStore.GetActiveOrganizationOrDefault()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if activeorg == nil {
			return fmt.Errorf("no org exist")
		}
		orgID = activeorg.ID
	} else {
		orgs, err := importStore.GetOrganizations(&store.GetOrganizationsOptions{Name: orgflag})
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

	err := createWorkspace(t, newWorkspace, orgID, importStore, deps, detached)
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

func createWorkspace(t *terminal.Terminal, workspace NewWorkspace, orgID string, importStore ImportStore, deps *store.Dependencies, detached bool) error {
	t.Vprint("\nWorkspace is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	var options *store.CreateWorkspacesOptions
	if deps != nil {
		setupscript1, err := setupscript.GenSetupHunkForLanguage("go", deps.Go)
		if err != nil {
			options = store.NewCreateWorkspacesOptions(clusterID, workspace.Name).WithGitRepo(workspace.GitRepo)
		} else {
			// setupscript2,err := setupscript.GenSetupHunkForLanguage("rust", deps.R)
			options = store.NewCreateWorkspacesOptions(clusterID, workspace.Name).WithGitRepo(workspace.GitRepo).WithStartupScript(setupscript1)
		}
	} else {
		options = store.NewCreateWorkspacesOptions(clusterID, workspace.Name).WithGitRepo(workspace.GitRepo)
	}

	user, err := importStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if featureflag.IsAdmin(user.GlobalUserType) {
		options.WorkspaceTemplateID = store.DevWorkspaceTemplateID
	}

	w, err := importStore.CreateWorkspace(orgID, options)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if detached {
		return nil
	} else {
		err = pollUntil(t, w.ID, "RUNNING", importStore, true)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		t.Vprint(t.Green("\nYour workspace is ready!"))
		t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.GetLocalIdentifier(nil)))
	}

	return nil
}

func pollUntil(t *terminal.Terminal, wsid string, state string, importStore ImportStore, canSafelyExit bool) error {
	s := t.NewSpinner()
	isReady := false
	if canSafelyExit {
		t.Vprintf("You can safely ctrl+c to exit\n")
	}
	s.Suffix = " hang tight 🤙"
	s.Start()
	for !isReady {
		time.Sleep(5 * time.Second)
		ws, err := importStore.GetWorkspace(wsid)
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

func isRust(t *terminal.Terminal, path string) bool {
	paths := recursivelyFindFile(t, []string{"Cargo\\.toml", "Cargo\\.lock"}, path)
	
	if len(paths) > 0 {
		return true
	}
	return false
}

func isNode(t *terminal.Terminal, path string) *string {
	paths := recursivelyFindFile(t, []string{"package\\-lock\\.json", "package\\.json", "node_modules"}, path)
	retval := ""
	if len(paths) > 0 {

		sort.Strings(paths)
		for _, path := range paths {
			fmt.Println(path)
			keypath := "engines.node"
			jsonstring, err := catFile(path)
			value := gjson.Get(jsonstring, "name")
			println(value.String())
			value = gjson.Get(jsonstring, keypath)
			println(value.String())

			if err != nil {
				//
			}
			if retval == "" {
				retval = value.String()
			}

		}
		return &retval
	}
	return nil
}

func getGoVersion(t *terminal.Terminal, path string) string {
	paths := recursivelyFindFile(t, []string{"go\\.mod"}, path)
	
	if len(paths) > 0 {

		sort.Strings(paths)
		for _, path := range paths {
			fmt.Println(path)
			res, err := readGoMod(path)
			if err != nil {
				//
			}
			return res
		}

		
	}
	return ""
}

func isRuby(t *terminal.Terminal, path string) bool {
	paths := recursivelyFindFile(t, []string{"Gemfile.lock", "Gemfile"}, path)
	
	if len(paths) > 0 {
		return true
	}
	return false
}

func isPython(t *terminal.Terminal, path string) bool {
	paths := recursivelyFindFile(t, []string{"Gemfile.lock", "Gemfile"}, path)
	
	if len(paths) > 0 {
		return true
	}
	return false
}

func appendPath(a string, b string) string {
	if a == "." {
		return b
	}
	return a + "/" + b 
}

// Returns list of paths to file
func recursivelyFindFile(t *terminal.Terminal, filename_s []string, path string) []string {

	var paths []string

	files, err := ioutil.ReadDir(path)
    if err != nil {
        fmt.Println(err);
    }
 
    for _, f := range files {
		dir, err := os.Stat(appendPath(path, f.Name()))
		if err != nil {
			// fmt.Println(t.Red(err.Error()))
		} else {
			for _, filename := range filename_s {

				r, _ := regexp.Compile(filename)
				res := r.MatchString(f.Name())

				if res {
					// t.Vprint(t.Yellow(filename) + "---" + t.Yellow(path+f.Name()))
					paths = append(paths, appendPath(path, f.Name()))

					// fileContents, err := catFile(appendPath(path, f.Name()))
					// if err != nil {
					// 	// 
					// } 

	
					// TODO: read
					// if file has json, read the json
				}
			}

			if dir.IsDir() {
				paths = append(paths, recursivelyFindFile(t, filename_s, appendPath(path, f.Name()))...)
			}
		}
	}

	//TODO: make the list unique
	
	return paths
}

// 
// read from gomod
// read from json

func catFile(filePath string) (string, error) {
	gocmd := exec.Command("cat", filePath) // #nosec G204
	in, err := gocmd.Output()
	if err != nil {
		return "", breverrors.WrapAndTrace(err, "error reading file "+ filePath)
	} else {
		d := charmap.CodePage850.NewDecoder()
		out, err := d.Bytes(in)
		if err != nil {
			return "", breverrors.WrapAndTrace(err, "error reading file "+ filePath)
		}
		return string(out), nil
	}
}

func readGoMod(filePath string) (string, error) {
	contents, err := catFile(filePath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(contents, "\n")

	for _, line := range lines {
		if strings.Contains(line, "go ") {
			return strings.Split(line, " ")[1], nil
		}
	}

	return "", nil
}
