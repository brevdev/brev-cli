//go:build !codeanalysis

package mergeshells

import (
	"embed"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/brevdev/brev-cli/pkg/collections"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/tidwall/gjson"
	"golang.org/x/text/encoding/charmap"
)

//go:embed templates/*
var templateFs embed.FS

func GenerateShellScript(path string) string {
	deps := GetDependencies(path)
	fmt.Println(strings.Join([]string{"** Recognized dependencies ** ", strings.Join(deps, " ")}, " "))
	return DependenciesToShell("bash", deps...)
}

func DependenciesToShell(shell string, deps ...string) string {
	shellString := strings.Join([]string{filepath.Join("#!/bin/", shell), MergeShells(deps...)}, "\n")
	return shellString
}

func GetDependencies(path string) []string {
	var deps []string

	// to add a new recognizer for a language,
	// write a method that returns *string,
	// where nil means this does not recognize
	// "" means use default
	// and any non-empty string returned is a version number constraint for us to use
	// then add it to the dictionary below under the same key
	// as the folder you can find the installer within
	// when looking under the templates/ directory.
	// if there is or a pre-existing recognizer, append it to the end of the {} list
	// if there is not a pre-existing recognizer, make certain to
	// 1) add a folder of the same name under templates/
	// 2) add any direct versions we support using their version number as file name
	// 3) add a 'generic' fallback if we don't recognize a specific version, using key as filename
	// TODO -- if there is no specific version installer for the version found
	//         then try to replace ${version} in the generic installer
	//              with the version number before importing it
	supportedDependencyMap := map[string][]func(string) *string{
		"node":   {nodeVersion},
		"gatsby": {gatsbyVersion},
		"rust":   {rustVersion},
		"golang": {goVersion},
	}

	for dep, fs := range supportedDependencyMap {
		versions := collections.Filter(func(x *string) bool {
			if x == nil {
				return false
			}
			return true
		}, collections.Fanout(fs, path))
		if len(versions) > 0 {
			version := versions[0]
			if len(*version) > 0 {
				dep = strings.Join([]string{dep, *version}, "-")
			}
			deps = append(deps, dep)
		}
	}

	return deps
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

type MergeShellStore interface {
	GetFileAsString(path string) (string, error)
}

func ImportPath(t *terminal.Terminal, path string, store MergeShellStore) {
	pathExists := dirExists(path)
	if !pathExists {
		fmt.Println(strings.Join([]string{"Path:", path, "does not exist."}, " "))
		return
	}
	var gitpath string
	if path == "." {
		gitpath = filepath.Join(".git", "config")
	} else {
		gitpath = filepath.Join(path, ".git", "config")
	}
	file, error := store.GetFileAsString(gitpath)
	if error != nil {
		fmt.Println(strings.Join([]string{"Could not read .git/config at", path}, " "))
		return
	}
	// Get GitUrl
	var gitURL string
	for _, v := range strings.Split(file, "\n") {
		if strings.Contains(v, "url") {
			gitURL = strings.Split(v, "= ")[1]
		}
	}
	if len(gitURL) == 0 {
		fmt.Println("no git url found")
		return
	}
	WriteBrevFile(t, GetDependencies(path), gitURL, path)
}

func WriteBrevFile(t *terminal.Terminal, deps []string, gitURL string, path string) *error {
	fmt.Println("\n\nGitUrl: ", gitURL)

	t.Vprint(t.Green("generating script with the following dependencies and writing resulting script to .brev/setup.sh : ", strings.Join(deps, " ")))
	// TODO
	// mk directory .brev if it does not already exist
	shellString := GenerateShellScript(path)
	mderr := os.MkdirAll(filepath.Join(path, ".brev"), os.ModePerm)
	if mderr == nil {
		// generate a string that is the collections.Concatenation of dependency-ordering the contents of all the dependencies
		// found by cat'ing the directory generated from the deps string, using the translated ruby code with go generics

		// place within .brev and write setup.sh with the result of this
		f, err := os.Create(filepath.Join(path, ".brev", "setup.sh"))
		if err != nil {
			return &err
		}

		defer f.Close()
		f.WriteString(shellString)
		f.Sync()
	} else {
		return &mderr
	}
	fmt.Println("\n\nGitUrl: ", gitURL)

	t.Vprint(t.Green("generating script witth the following dependencies and writing resulting script to .brev/setup.sh : ", strings.Join(deps, " ")))
	return nil
}

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
	return collections.Foldl(func(acc map[string]ShellFragment, frag ShellFragment) map[string]ShellFragment { //nolint:typecheck
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
	// collections.Foldl lines pulling out parsable information and occasionally pushing onto script so far and generating a new fragment, if need be
	acc := collections.Foldl(func(acc scriptAccumulator, line string) scriptAccumulator { //nolint:typecheck
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
	noversion := false
	if len(subPaths) == 1 {
		noversion = true
		subPaths = collections.Duplicate(subPaths[0])
	}
	path := filepath.Join(collections.Concat([]string{"templates"}, subPaths)...) //nolint:typecheck
	script, err := templateFs.Open(path)
	if err != nil && !noversion {
		if subPaths[0] != subPaths[1] {
			return importFile(subPaths[0]) //nolint:typecheck
		} else {
			return []ShellFragment{}, errors.New(strings.Join([]string{"Path does not exist:", path}, " "))
		}
	}
	out, err := ioutil.ReadAll(script)
	stringScript := string(out)
	fmt.Println(stringScript)
	if err != nil {
		return []ShellFragment{}, err
	}
	return fromSh(stringScript), nil
}

func toSh(script []ShellFragment) string {
	// collections.Flatmap across generating the script from all of the component shell bits
	return strings.Join(collections.Flatmap(func(frag ShellFragment) []string { //nolint:typecheck
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
	return collections.Flatmap(func(dep_name string) []string { //nolint:typecheck
		return append(findDependencies(dep_name, baselineDependencies, globalDependencies), dep_name)
	}, dependencies)
}

func splitIntoLibraryAndSeq(shFragments []ShellFragment) ([]string, map[string]ShellFragment, []string) {
	return collections.Fmap(func(some ShellFragment) string { //nolint:typecheck
		if some.Name != nil {
			return *some.Name
		} else {
			return ""
		}
	}, shFragments), toDefsDict(shFragments), []string{}
}

func prependDependencies(shName string, baselineDependencies map[string]ShellFragment, globalDependencies map[string]ShellFragment) OrderDefsFailures {
	dependencies := collections.Uniq(findDependencies(shName, baselineDependencies, globalDependencies)) //nolint:typecheck
	// baseline_deps := collections.Filter(func (dep string) bool {
	// 	if _, ok := baselineDependencies[dep]; ok {
	// 		return true
	// 	}
	// 	return false
	// }, dependencies)
	nonBaselineDependencies := collections.Filter(func(dep string) bool { //nolint:typecheck
		if _, ok := baselineDependencies[dep]; ok {
			return false
		}
		return true
	}, dependencies)
	canBeFixedDependencies := collections.Filter(func(dep string) bool { //nolint:typecheck
		if _, ok := globalDependencies[dep]; ok {
			return true
		}
		return false
	}, nonBaselineDependencies)
	cantBeFixedDependencies := collections.Difference(nonBaselineDependencies, canBeFixedDependencies) //nolint:typecheck

	addedBaselineDependencies := collections.Foldl( //nolint:typecheck
		func(deps map[string]ShellFragment, dep string) map[string]ShellFragment {
			deps[dep] = globalDependencies[dep]
			return deps
		}, map[string]ShellFragment{}, canBeFixedDependencies)
	return OrderDefsFailures{Order: append(dependencies, shName), Defs: collections.DictMerge(addedBaselineDependencies, baselineDependencies), Failures: cantBeFixedDependencies} //nolint:typecheck
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
	newestOrderDefsFailures := collections.Foldl(func(acc OrderDefsFailures, shName string) OrderDefsFailures { //nolint:typecheck
		newOrderDefsFailures := prependDependencies(shName, acc.Defs, globalDependencies)
		return OrderDefsFailures{Order: append(collections.Concat(acc.Order, newOrderDefsFailures.Order), shName), Failures: collections.Concat(acc.Failures, newOrderDefsFailures.Failures), Defs: collections.DictMerge(acc.Defs, newOrderDefsFailures.Defs)} //nolint:typecheck
	}, OrderDefsFailures{Order: []string{}, Failures: []string{}, Defs: shellDefs}, order)
	return OrderDefsFailures{Order: collections.Uniq(newestOrderDefsFailures.Order), Defs: newestOrderDefsFailures.Defs, Failures: collections.Uniq(collections.Concat(failures, newestOrderDefsFailures.Failures))} //nolint:typecheck
}

func process(shFragments []ShellFragment, baselineDependencies map[string]ShellFragment, globalDependencies map[string]ShellFragment) []ShellFragment {
	resultOrderDefsFailures := addDependencies(shFragments, baselineDependencies, globalDependencies)
	// TODO print or return the failed installation instructions "FAILED TO FIND INSTALLATION INSTRUCTIONS FOR: #{failures}" if failures.length > 0
	return collections.Fmap(func(x string) ShellFragment { //nolint:typecheck
		return resultOrderDefsFailures.Defs[x]
	}, resultOrderDefsFailures.Order)
}

func MergeShells(deps ...string) string {
	importedDependencies := collections.Flatmap(func(dep string) []ShellFragment { //nolint:typecheck
		frags, err := importFile(dep)
		if err != nil {
			return []ShellFragment{}
		} else {
			return frags
		}
	}, deps)
	processedDependencies := process(importedDependencies, map[string]ShellFragment{}, map[string]ShellFragment{})
	shellScript := toSh(processedDependencies)
	return shellScript
}

func rustVersion(path string) *string {
	paths := recursivelyFindFile([]string{"Cargo\\.toml", "Cargo\\.lock"}, path)

	if len(paths) > 0 {
		retval := ""
		return &retval
	}
	return nil
}

func nodeVersion(path string) *string {
	paths := recursivelyFindFile([]string{"package\\-lock\\.json$", "package\\.json$"}, path)
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

func gatsbyVersion(path string) *string {
	paths := recursivelyFindFile([]string{"package\\.json"}, path)
	retval := ""
	var foundGatsby bool
	if len(paths) > 0 {

		sort.Strings(paths)
		for _, path := range paths {
			keypath := "dependencies.gatsby"
			jsonstring, err := files.CatFile(path)
			value := gjson.Get(jsonstring, keypath)

			if err != nil {
				//
			}
			if value.String() != "" {
				foundGatsby = true
			}

		}
		if foundGatsby {
			return &retval
		}
		return nil
	}
	return nil
}

func goVersion(path string) *string {
	paths := recursivelyFindFile([]string{"go\\.mod"}, path)

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

func isRuby(path string) bool {
	paths := recursivelyFindFile([]string{"Gemfile.lock", "Gemfile"}, path)

	if len(paths) > 0 {
		return true
	}
	return false
}

func isPython(path string) bool {
	paths := recursivelyFindFile([]string{"Gemfile.lock", "Gemfile"}, path)

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
func recursivelyFindFile(filenames []string, path string) []string {
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
				paths = append(paths, recursivelyFindFile(filenames, appendPath(path, f.Name()))...)
			}
		}
	}

	// TODO: make the list collections.Unique

	return paths
}

//
// read from gomod
// read from json

func CatFile(filePath string) (string, error) {
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
