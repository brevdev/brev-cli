package importpkg

import (
	"embed" // so we don't have to call embed since it's an internal generator that gets run when we compile
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/setupscript"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	"golang.org/x/text/encoding/charmap"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

//go:embed templates/*
var templateFs embed.FS

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
	s := t.NewSpinner()
	s.Suffix = " Reading your code for dependencies 🤙"
	s.Start()

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

	var deps []string

	node := isNode(t, path)
	if node != nil && len(*node) > 0 {
		deps = append(deps, "node-"+*node)
	} else if node != nil && len(*node) == 0 {
		deps = append(deps, "node-14")
	}

	gatsby := isGatsby(t, path)
	if gatsby != nil {
		deps = append(deps, "gatsby")
	}

	rust := isRust(t, path)
	if rust {
		deps = append(deps, "rust")
	}

	golang := getGoVersion(t, path)
	if golang == nil {
		_ = 0
	} else if *golang == "" {
		deps = append(deps, "golang-1.17")
	} else {
		deps = append(deps, "golang-"+*golang)
	}

	s.Stop()

	fmt.Println("\n\nGitUrl: ", gitURL)

	// example of using generic Filter function on a list of strings
	test := Filter(func(x string) bool { //nolint:typecheck
		return strings.HasPrefix(x, "n")
	}, []string{"foo", "bar", "naranj", "nobody"})
	t.Vprint(t.Green(strings.Join(test, " ")))

	// example of duplicating every element of a list using generic duplicate and Flatmap
	t.Vprint(t.Green(strings.Join(Flatmap(duplicate[string], test), " "))) //nolint:typecheck
	result := strings.Join(deps, " ")

	t.Vprint(t.Green("./merge-shells.rb %s", result))
	// TODO
	// mk directory .brev if it does not already exist

	mderr := os.MkdirAll(".brev", os.ModePerm)
	if mderr == nil {
		// generate a string that is the Concatenation of dependency-ordering the contents of all the dependencies
		// found by cat'ing the directory generated from the deps string, using the translated ruby code with go generics
		shellString := mergeShells(deps...)
		// place within .brev and write setup.sh with the result of this
		f, err := os.Create(filepath.Join(".brev", "auto-setup.sh"))
		if err != nil {
			return err
		}

		defer f.Close()
		f.WriteString(shellString)
		f.Sync()
	} else {
		return mderr
	}

	// err = clone(t, gitURL, orgflag, importStore, name, deps, detached)
	// if err != nil {
	// 	return breverrors.WrapAndTrace(err)
	// }

	return nil
}

// func mergeShells(dependencies ...string) string {
// 	return strings.Join(dependencies, " ")
// }

type ShellFragment struct {
	Name         *string  `json:"name"`
	Tag          *string  `json:"tag"`
	Comment      *string  `json:"comment"`
	Script       []string `json:"script"`
	Dependencies []string `json:"dependencies"`
}

func parseCommentLine(line string) ShellFragment {
	commentFreeLine := strings.TrimSpace(strings.ReplaceAll(line, "#", ""))
	parts := strings.Split(commentFreeLine, " ")

	if strings.HasPrefix(parts[0], "dependencies") {
		temp := parts[1:]
		return ShellFragment{Dependencies: temp}
	}

	if len(parts) == 2 {
		return ShellFragment{Name: &parts[0], Tag: &parts[1]}
	} else if len(parts) == 1 {
		return ShellFragment{Name: &parts[0]}
	} else {
		return ShellFragment{Comment: &commentFreeLine}
	}
}

func toDefsDict(shFragment []ShellFragment) map[string]ShellFragment {
	return Foldl(func(acc map[string]ShellFragment, frag ShellFragment) map[string]ShellFragment { //nolint:typecheck
		if frag.Name != nil {
			acc[*frag.Name] = frag
		}
		return acc
	}, map[string]ShellFragment{}, shFragment)
}

type scriptAccumulator struct {
	CurrentFragment ShellFragment
	ScriptSoFar     []ShellFragment
}

func fromSh(shScript string) []ShellFragment {
	// get lines
	lines := strings.Split(shScript, "\n")
	base := scriptAccumulator{}
	// Foldl lines pulling out parsable information and occasionally pushing onto script so far and generating a new fragment, if need be
	acc := Foldl(func(acc scriptAccumulator, line string) scriptAccumulator { //nolint:typecheck
		if strings.HasPrefix(line, "#") { // then it is a comment string, which may or may not have parsable information
			if len(acc.CurrentFragment.Script) > 0 {
				acc.ScriptSoFar = append(acc.ScriptSoFar, acc.CurrentFragment)
				acc.CurrentFragment = ShellFragment{}
			}
			parsed := parseCommentLine(line)
			if parsed.Name != nil {
				acc.CurrentFragment.Name = parsed.Name
			}
			if parsed.Tag != nil {
				acc.CurrentFragment.Tag = parsed.Tag
			}
			if parsed.Comment != nil {
				acc.CurrentFragment.Comment = parsed.Comment
			}
			if parsed.Dependencies != nil {
				acc.CurrentFragment.Dependencies = parsed.Dependencies
			}
		} else {
			acc.CurrentFragment.Script = append(acc.CurrentFragment.Script, line)
		}
		return acc
	}, base, lines)
	// return the script so far plus the last fragment remaining after folding
	return append(acc.ScriptSoFar, acc.CurrentFragment)
}

func importFile(nameVersion string) ([]ShellFragment, error) {
	// split the name string into two with - -- left hand side is package, right hand side is version
	// read from the generated path
	// generate ShellFragment from it (fromSh) and return it
	subPaths := strings.Split(nameVersion, "-")
	if len(subPaths) == 1 {
		subPaths = duplicate(subPaths[0])
	}
	path := filepath.Join(Concat([]string{"templates"}, subPaths)...) //nolint:typecheck
	script, err := templateFs.Open(path)
	out, err := ioutil.ReadAll(script)
	stringScript := string(out)
	fmt.Println(stringScript)
	if err != nil {
		return []ShellFragment{}, err
	}
	return fromSh(stringScript), nil
}

func toSh(script []ShellFragment) string {
	// Flatmap across generating the script from all of the component shell bits
	return strings.Join(Flatmap(func(frag ShellFragment) []string { //nolint:typecheck
		name, tag, comment := frag.Name, frag.Tag, frag.Comment
		innerScript, dependencies := frag.Script, frag.Dependencies
		returnval := []string{}
		if name != nil {
			nameTag := strings.Join([]string{"#", *name}, " ")
			if tag != nil {
				nameTag = strings.Join([]string{nameTag, *tag}, " ")
			}
			returnval = append(returnval, nameTag)
		}

		if comment != nil {
			returnval = append(returnval, *comment)
		}
		if len(dependencies) > 0 {
			returnval = append(returnval, strings.Join(dependencies, " "))
		}
		returnval = append(returnval, strings.Join(innerScript, "\n"))
		return returnval
	}, script), "\n")
}

func findDependencies(shName string, baselineDependencies map[string]ShellFragment, globalDependencies map[string]ShellFragment) []string {
	definition := ShellFragment{}
	if val, ok := baselineDependencies[shName]; ok {
		definition = val
	} else if val2, ok2 := globalDependencies[shName]; ok2 {
		definition = val2
	}

	dependencies := definition.Dependencies
	return Flatmap(func(dep_name string) []string { //nolint:typecheck
		return append(findDependencies(dep_name, baselineDependencies, globalDependencies), dep_name)
	}, dependencies)
}

func splitIntoLibraryAndSeq(shFragments []ShellFragment) ([]string, map[string]ShellFragment, []string) {
	return Fmap(func(some ShellFragment) string { //nolint:typecheck
		if some.Name != nil {
			return *some.Name
		} else {
			return ""
		}
	}, shFragments), toDefsDict(shFragments), []string{}
}

func prependDependencies(shName string, baselineDependencies map[string]ShellFragment, globalDependencies map[string]ShellFragment) OrderDefsFailures {
	dependencies := Uniq(findDependencies(shName, baselineDependencies, globalDependencies)) //nolint:typecheck
	// baseline_deps := Filter(func (dep string) bool {
	// 	if _, ok := baselineDependencies[dep]; ok {
	// 		return true
	// 	}
	// 	return false
	// }, dependencies)
	nonBaselineDependencies := Filter(func(dep string) bool { //nolint:typecheck
		if _, ok := baselineDependencies[dep]; ok {
			return false
		}
		return true
	}, dependencies)
	canBeFixedDependencies := Filter(func(dep string) bool { //nolint:typecheck
		if _, ok := globalDependencies[dep]; ok {
			return true
		}
		return false
	}, nonBaselineDependencies)
	cantBeFixedDependencies := Difference(nonBaselineDependencies, canBeFixedDependencies) //nolint:typecheck

	addedBaselineDependencies := Foldl( //nolint:typecheck
		func(deps map[string]ShellFragment, dep string) map[string]ShellFragment {
			deps[dep] = globalDependencies[dep]
			return deps
		}, map[string]ShellFragment{}, canBeFixedDependencies)
	return OrderDefsFailures{Order: append(dependencies, shName), Defs: DictMerge(addedBaselineDependencies, baselineDependencies), Failures: cantBeFixedDependencies} //nolint:typecheck
}

type OrderDefsFailures struct {
	Order    []string
	Defs     map[string]ShellFragment
	Failures []string
}

func addDependencies(shFragments []ShellFragment, baselineDependencies map[string]ShellFragment, globalDependencies map[string]ShellFragment) OrderDefsFailures {
	//     ## it's a left fold across the importFile statements
	//     ## at any point, if the dependencies aren't already in the loaded dictionary
	//     ## we add them before we add the current item, and we add them and the current item into the loaded dictionary
	order, shellDefs, failures := splitIntoLibraryAndSeq(shFragments)
	newestOrderDefsFailures := Foldl(func(acc OrderDefsFailures, shName string) OrderDefsFailures { //nolint:typecheck
		newOrderDefsFailures := prependDependencies(shName, acc.Defs, globalDependencies)
		return OrderDefsFailures{Order: append(Concat(acc.Order, newOrderDefsFailures.Order), shName), Failures: Concat(acc.Failures, newOrderDefsFailures.Failures), Defs: DictMerge(acc.Defs, newOrderDefsFailures.Defs)} //nolint:typecheck
	}, OrderDefsFailures{Order: []string{}, Failures: []string{}, Defs: shellDefs}, order)
	return OrderDefsFailures{Order: Uniq(newestOrderDefsFailures.Order), Defs: newestOrderDefsFailures.Defs, Failures: Uniq(Concat(failures, newestOrderDefsFailures.Failures))} //nolint:typecheck
}

func process(shFragments []ShellFragment, baselineDependencies map[string]ShellFragment, globalDependencies map[string]ShellFragment) []ShellFragment {
	resultOrderDefsFailures := addDependencies(shFragments, baselineDependencies, globalDependencies)
	// TODO print or return the failed installation instructions "FAILED TO FIND INSTALLATION INSTRUCTIONS FOR: #{failures}" if failures.length > 0
	return Fmap(func(x string) ShellFragment { //nolint:typecheck
		return resultOrderDefsFailures.Defs[x]
	}, resultOrderDefsFailures.Order)
}

func mergeShells(filepaths ...string) string {
	return toSh(process(Flatmap(func(path string) []ShellFragment { //nolint:typecheck
		frags, err := importFile(path)
		if err != nil {
			return []ShellFragment{}
		} else {
			return frags
		}
	}, filepaths), map[string]ShellFragment{}, map[string]ShellFragment{}))
}

func duplicate[T any](x T) []T {
	return []T{x, x}
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
	paths := recursivelyFindFile(t, []string{"package\\-lock\\.json$", "package\\.json$"}, path)
	retval := ""
	if len(paths) > 0 {

		sort.Strings(paths)

		i := len(paths) - 1

		keypath := "engines.node"
		jsonstring, err := files.CatFile(paths[i])
		value := gjson.Get(jsonstring, "name")
		value = gjson.Get(jsonstring, keypath)

		if err != nil {
			//
		}
		if retval == "" {
			retval = value.String()
		}

		return &retval
	}
	return nil
}

func isGatsby(t *terminal.Terminal, path string) *string {
	paths := recursivelyFindFile(t, []string{"package\\.json"}, path)
	retval := ""
	if len(paths) > 0 {

		sort.Strings(paths)
		for _, path := range paths {
			keypath := "dependencies.gatsby"
			jsonstring, err := files.CatFile(path)
			value := gjson.Get(jsonstring, keypath)

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

func getGoVersion(t *terminal.Terminal, path string) *string {
	paths := recursivelyFindFile(t, []string{"go\\.mod"}, path)

	if len(paths) > 0 {

		sort.Strings(paths)
		for _, path := range paths {
			fmt.Println(path)
			res, err := readGoMod(path)
			if err != nil {
				//
			}
			return &res
		}

	}
	return nil
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
func recursivelyFindFile(t *terminal.Terminal, filenames []string, path string) []string {
	var paths []string

	files, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println(err)
	}

	for _, f := range files {
		dir, err := os.Stat(appendPath(path, f.Name()))
		if err != nil {
			// fmt.Println(t.Red(err.Error()))
		} else {
			for _, filename := range filenames {

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
				paths = append(paths, recursivelyFindFile(t, filenames, appendPath(path, f.Name()))...)
			}
		}
	}

	// TODO: make the list Unique

	return paths
}

//
// read from gomod
// read from json

func catFile(filePath string) (string, error) {
	gocmd := exec.Command("cat", filePath) // #nosec G204
	in, err := gocmd.Output()
	if err != nil {
		return "", breverrors.WrapAndTrace(err, "error reading file "+filePath)
	} else {
		d := charmap.CodePage850.NewDecoder()
		out, err := d.Bytes(in)
		if err != nil {
			return "", breverrors.WrapAndTrace(err, "error reading file "+filePath)
		}
		return string(out), nil
	}
}

func readGoMod(filePath string) (string, error) {
	contents, err := files.CatFile(filePath)
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
