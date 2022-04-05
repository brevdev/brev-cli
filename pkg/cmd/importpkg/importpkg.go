package importpkg

import (
	_ "embed"
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

	gatsby := isGatsby(t, path)
	if gatsby != nil {
		deps = append(deps, "gatsby")
	}

	node := isNode(t, path)
	if node != nil && len(*node) > 0 {
		deps = append(deps, "node-"+*node)
	} else if node != nil && len(*node) == 0 {
		deps = append(deps, "node-14")
	}

	rust := isRust(t, path)
	if rust {
		deps = append(deps, "rust")
	}

	golang := getGoVersion(t, path)
	if golang == "" {
		deps = append(deps, "golang-1.17")
	} else {
		deps = append(deps, "golang-"+golang)
	}

	s.Stop()

	fmt.Println("\n\nGitUrl: ", gitURL)

	// example of using generic filter function on a list of strings
	test := filter(func(x string) bool {
		return strings.HasPrefix(x, "n")
	}, []string{"foo", "bar", "naranj", "nobody"})
	t.Vprint(t.Green(strings.Join(test, " ")))

	// example of duplicating every element of a list using generic duplicate and flatmap
	t.Vprint(t.Green(strings.Join(flatmap(func(x string) []string { return duplicate(x) }, test), " ")))
	result := strings.Join(deps, " ")

	t.Vprint(t.Green("./merge-shells.rb %s", result))
	// TODO
	// mk directory .brev if it does not already exist

	mderr := os.MkdirAll(".brev", os.ModePerm)
	if mderr == nil {
		// generate a string that is the concatenation of dependency-ordering the contents of all the dependencies
		// found by cat'ing the directory generated from the deps string, using the translated ruby code with go generics
		shell_string := merge_shells(deps...)
		// place within .brev and write setup.sh with the result of this
		f, err := os.Create(filepath.Join(".brev", "auto-setup.sh"))
		if err != nil {
			return err
		}

		defer f.Close()
		f.WriteString(shell_string)
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

// func merge_shells(dependencies ...string) string {
// 	return strings.Join(dependencies, " ")
// }

type ShellFragment struct {
	Name         *string  `json:"name"`
	Tag          *string  `json:"tag"`
	Comment      *string  `json:"comment"`
	Script       []string `json:"script"`
	Dependencies []string `json:"dependencies"`
}

func parse_comment_line(line string) ShellFragment {
	comment_free_line := strings.Replace(line, "#", "", -1)
	parts := strings.Split(comment_free_line, " ")

	if strings.HasPrefix(parts[0], "dependencies") {
		temp := parts[1:]
		return ShellFragment{Dependencies: temp}
	}

	if len(parts) == 2 {
		return ShellFragment{Name: &parts[0], Tag: &parts[1]}
	} else if len(parts) == 1 {
		return ShellFragment{Name: &parts[0]}
	} else {
		return ShellFragment{Comment: &comment_free_line}
	}
}

func to_defs_dict(sh_fragment []ShellFragment) map[string]ShellFragment {
	return foldl(func(acc map[string]ShellFragment, frag ShellFragment) map[string]ShellFragment {
		if frag.Name != nil {
			acc[*frag.Name] = frag
		}
		return acc
	}, map[string]ShellFragment{}, sh_fragment)
}

type script_accumulator struct {
	CurrentFragment ShellFragment
	ScriptSoFar     []ShellFragment
}

func from_sh(sh_script string) []ShellFragment {
	// get lines
	lines := strings.Split(sh_script, "\n")
	base := script_accumulator{}
	// foldl lines pulling out parsable information and occasionally pushing onto script so far and generating a new fragment, if need be
	acc := foldl(func(acc script_accumulator, line string) script_accumulator {
		if strings.HasPrefix(line, "#") { // then it is a comment string, which may or may not have parsable information
			if len(acc.CurrentFragment.Script) > 0 {
				acc.ScriptSoFar = append(acc.ScriptSoFar, acc.CurrentFragment)
				acc.CurrentFragment = ShellFragment{}
			}
			parsed := parse_comment_line(line)
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

func import_file(name_version string) ([]ShellFragment, error) {
	// split the name string into two with - -- left hand side is package, right hand side is version
	// read from the generated path
	// generate ShellFragment from it (from_sh) and return it
	sub_paths := strings.Split(name_version, "-")
	path := filepath.Join(sub_paths...)
	script, err := catFile(path)
	if err != nil {
		return []ShellFragment{}, err
	}
	return from_sh(script), nil
}

func to_sh(script []ShellFragment) string {
	// flatmap across generating the script from all of the component shell bits
	return strings.Join(flatmap(func(frag ShellFragment) []string {
		name, tag, comment := frag.Name, frag.Tag, frag.Comment
		inner_script, dependencies := frag.Script, frag.Dependencies
		returnval := []string{}
		if name != nil {
			name_tag := strings.Join([]string{"#", *name}, " ")
			if tag != nil {
				name_tag = strings.Join([]string{name_tag, *tag}, " ")
			}
			returnval = append(returnval, name_tag)
		}

		if comment != nil {
			returnval = append(returnval, *comment)
		}
		if len(dependencies) > 0 {
			returnval = append(returnval, strings.Join(dependencies, " "))
		}
		returnval = append(returnval, strings.Join(inner_script, "\n"))
		return returnval
	}, script), "\n")
}

func find_dependencies(sh_name string, baseline_dependencies map[string]ShellFragment, global_dependencies map[string]ShellFragment) []string {
	definition := ShellFragment{}
	if val, ok := baseline_dependencies[sh_name]; ok {
		definition = val
	} else if val2, ok2 := global_dependencies[sh_name]; ok2 {
		definition = val2
	}

	dependencies := definition.Dependencies
	return flatmap(func(dep_name string) []string {
		return append(find_dependencies(dep_name, baseline_dependencies, global_dependencies), dep_name)
	}, dependencies)
}

func split_into_library_and_seq(sh_fragments []ShellFragment) ([]string, map[string]ShellFragment, []string) {
	return fmap(func(some ShellFragment) string {
		if some.Name != nil {
			return *some.Name
		} else {
			return ""
		}
	}, sh_fragments), to_defs_dict(sh_fragments), []string{}
}

func prepend_dependencies(sh_name string, baseline_dependencies map[string]ShellFragment, global_dependencies map[string]ShellFragment) order_defs_failures {
	dependencies := uniq(find_dependencies(sh_name, baseline_dependencies, global_dependencies))
	// baseline_deps := filter(func (dep string) bool {
	// 	if _, ok := baseline_dependencies[dep]; ok {
	// 		return true
	// 	}
	// 	return false
	// }, dependencies)
	non_baseline_dependencies := filter(func(dep string) bool {
		if _, ok := baseline_dependencies[dep]; ok {
			return false
		}
		return true
	}, dependencies)
	can_be_fixed_dependencies := filter(func(dep string) bool {
		if _, ok := global_dependencies[dep]; ok {
			return true
		}
		return false
	}, non_baseline_dependencies)
	cant_be_fixed_dependencies := difference(non_baseline_dependencies, can_be_fixed_dependencies)

	added_baseline_dependencies := foldl(
		func(deps map[string]ShellFragment, dep string) map[string]ShellFragment {
			deps[dep] = global_dependencies[dep]
			return deps
		}, map[string]ShellFragment{}, can_be_fixed_dependencies)
	return order_defs_failures{Order: append(dependencies, sh_name), Defs: dict_merge(added_baseline_dependencies, baseline_dependencies), Failures: cant_be_fixed_dependencies}
}

type order_defs_failures struct {
	Order    []string
	Defs     map[string]ShellFragment
	Failures []string
}

func add_dependencies(sh_fragments []ShellFragment, baseline_dependencies map[string]ShellFragment, global_dependencies map[string]ShellFragment) order_defs_failures {
	//     ## it's a left fold across the import_file statements
	//     ## at any point, if the dependencies aren't already in the loaded dictionary
	//     ## we add them before we add the current item, and we add them and the current item into the loaded dictionary
	order, shell_defs, failures := split_into_library_and_seq(sh_fragments)
	newest_order_defs_failures := foldl(func(acc order_defs_failures, sh_name string) order_defs_failures {
		new_order_defs_failures := prepend_dependencies(sh_name, acc.Defs, global_dependencies)
		return order_defs_failures{Order: append(concat(acc.Order, new_order_defs_failures.Order), sh_name), Failures: concat(acc.Failures, new_order_defs_failures.Failures), Defs: new_order_defs_failures.Defs}
	}, order_defs_failures{Order: []string{}, Failures: []string{}, Defs: shell_defs}, order)
	return order_defs_failures{Order: uniq(newest_order_defs_failures.Order), Defs: newest_order_defs_failures.Defs, Failures: uniq(concat(failures, newest_order_defs_failures.Failures))}
}

func process(sh_fragments []ShellFragment, baseline_dependencies map[string]ShellFragment, global_dependencies map[string]ShellFragment) []ShellFragment {
	result_order_defs_failures := add_dependencies(sh_fragments, baseline_dependencies, global_dependencies)
	// TODO print or return the failed installation instructions "FAILED TO FIND INSTALLATION INSTRUCTIONS FOR: #{failures}" if failures.length > 0
	return fmap(func(x string) ShellFragment {
		return result_order_defs_failures.Defs[x]
	}, result_order_defs_failures.Order)
}

func merge_shells(filepaths ...string) string {
	return to_sh(process(flatmap(func(path string) []ShellFragment {
		frags, err := import_file(path)
		if err != nil {
			return frags
		} else {
			return []ShellFragment{}
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

		i := len(paths) - 1

		keypath := "engines.node"
		jsonstring, err := catFile(paths[i])
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
		fmt.Println(err)
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

	// TODO: make the list unique

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

func foldl[T any, R any](fn func(acc R, next T) R, base R, list []T) R {
	for _, value := range list {
		base = fn(base, value)
	}

	return base
}

func foldr[T any, R any](fn func(next T, carry R) R, base R, list []T) R {
	for idx := len(list) - 1; idx >= 0; idx-- {
		base = fn(list[idx], base)
	}

	return base
}

func concat[T any](left []T, right []T) []T {
	return foldl(func(acc []T, next T) []T {
		return append(acc, next)
	}, left, right)
}

func fmap[T any, R any](fn func(some T) R, list []T) []R {
	return foldl(func(acc []R, next T) []R {
		return append(acc, fn(next))
	}, []R{}, list)
}

func filter[T any](fn func(some T) bool, list []T) []T {
	return foldl(func(acc []T, next T) []T {
		if fn(next) {
			acc = append(acc, next)
		}
		return acc
	}, []T{}, list)
}

func flatmap[T any, R any](fn func(some T) []R, list []T) []R {
	return foldl(func(acc []R, el T) []R {
		return concat(acc, fn(el))
	}, []R{}, list)
}

// there is no function overloading [and the need to describe dependent relations between the types of the functions rules out variadic arguments]
// so we will define c2, c3, c4, and c5 which will allow simple composition of up to 5 functions
// anything more than that should be refactored so that subcomponents of the composition are renamed, anyway (or named itself)

func compose[T any, S any, R any](fn1 func(some S) R, fn2 func(some T) S) func(some T) R {
	return func(some T) R {
		return fn1(fn2(some))
	}
}

func c2[T any, S any, R any](fn1 func(some S) R, fn2 func(some T) S) func(some T) R {
	return compose(fn1, fn2)
}

func c3[T any, S any, R any, U any](fn0 func(some R) U, fn1 func(some S) R, fn2 func(some T) S) func(some T) U {
	return func(some T) U {
		return fn0(fn1(fn2(some)))
	}
}

func c4[T any, S any, R any, U any, V any](fn01 func(some U) V, fn0 func(some R) U, fn1 func(some S) R, fn2 func(some T) S) func(some T) V {
	return func(some T) V {
		return fn01(fn0(fn1(fn2(some))))
	}
}

func c5[T any, S any, R any, U any, V any, W any](fn02 func(some V) W, fn01 func(some U) V, fn0 func(some R) U, fn1 func(some S) R, fn2 func(some T) S) func(some T) W {
	return func(some T) W {
		return fn02(fn01(fn0(fn1(fn2(some)))))
	}
}

type maplist[T comparable] struct {
	List []T
	Map  map[T]bool
}

func uniq[T comparable](xs []T) []T {
	result := foldl(func(acc maplist[T], el T) maplist[T] {
		if _, ok := acc.Map[el]; !ok {
			acc.Map[el] = true
			acc.List = append(acc.List, el)
		}
		return acc
	}, maplist[T]{List: []T{}, Map: map[T]bool{}}, xs)
	return result.List
}

func to_dict[T comparable](xs []T) map[T]bool {
	return foldl(func(acc map[T]bool, el T) map[T]bool {
		acc[el] = true
		return acc
	}, map[T]bool{}, xs)
}

func difference[T comparable](from []T, remove []T) []T {
	returnval := foldl(func(acc maplist[T], el T) maplist[T] {
		if _, ok := acc.Map[el]; !ok {
			acc.Map[el] = true
			acc.List = append(acc.List, el)
		}
		return acc
	}, maplist[T]{Map: to_dict(remove), List: []T{}}, from)
	return returnval.List
}

func dict_merge[K comparable, V any](left map[K]V, right map[K]V) map[K]V {
	newMap := map[K]V{}
	for key, val := range left {
		if _, ok := right[key]; ok {
			newMap[key] = right[key]
		} else {
			newMap[key] = val
		}
	}
	return newMap
}
