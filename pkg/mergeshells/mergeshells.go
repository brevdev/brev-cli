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
)

//go:embed templates/*
var templateFs embed.FS

func GenerateShellScript(path string) string {
	deps := GetDependencies(path)
	// fmt.Println(strings.Join([]string{"** Recognized dependencies **", strings.Join(deps, " ")}, "\n"))
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

	// these will be applied, in left-to-right order, to the version string returned by your version function
	// before passing it to the finder / splicer of the install shell script for your dependency
	processVersionMap := map[string][]func(string) string{
		"golang":  {transformGoVersion},
		"default": {transformVersion},
	}

	for dep, fs := range supportedDependencyMap {
		versions := collections.Filter(collections.Fanout(fs, path), func(x *string) bool {
			return x != nil
		})
		if len(versions) > 0 {
			version := versions[0]
			if len(*version) > 0 {
				versionTransforms := processVersionMap["default"]
				if len(processVersionMap[dep]) > 0 {
					versionTransforms = processVersionMap[dep]
				}

				finalVersion := collections.S(versionTransforms...)(*version)
				dep = strings.Join([]string{dep, finalVersion}, "-")
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
	file, err := store.GetFileAsString(gitpath)
	if err != nil {
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
	if !dirExists(filepath.Join(path, ".brev", "setup.sh")) {
		_ = WriteBrevFile(t, GetDependencies(path), gitURL, path)
	} else {
		fmt.Println(".brev/setup.sh already exists - will not overwrite.")
	}
}

func GenerateLogs(script string) string {
	fragments := fromSh(script)
	return strings.Join(collections.Fmap(extractInstallLine, fragments), "\n")
}

func extractInstallLine(fragment ShellFragment) string {
	line := ""
	if fragment.Comment != nil {
		line = strings.Join([]string{"*", *fragment.Comment}, " ")
	}
	return line
}

func transformGoVersion(version string) string {
	// in Ruby:
	// `curl https://go.dev/dl/`.scan(/id=\"go([\w\.]+)\"/).flat_map{|x| x}.filter{|x| x.start_with?(version)}[0]
	if strings.Count(version, ".") < 2 {
		// if this is not a well-specified version
		// then get the list of versions from the official go website
		out, err := exec.Command("curl", "https://go.dev/dl/").Output()
		if err != nil {
			fmt.Println("Issue fetching go version, defaulting to same as input")
			return version
		}
		value := string(out)
		// pull out the go versions from the ids of the subsections of the download page
		re := regexp.MustCompile("id=\"go([\\w\\.]+)\"")
		versions := re.FindAllString(value, 1000)
		versions = collections.Fmap(func(x string) string { return x[6 : len(x)-1] }, versions)
		prefixFn := collections.P2(collections.Flip(strings.HasPrefix), version)
		// and fetch the first version that matches the prefix version you are looking for
		// as it is rendered on the go page in reverse chronological order
		// of nearest release to farthest for each version
		matchingVersion := collections.First(collections.Filter(versions, prefixFn))
		if matchingVersion == nil {
			fmt.Println(strings.Join([]string{"No stable version found for", version}, " "))
			// no stable version found so return the original version
			return version
		}
		return *matchingVersion
	}
	return version
}

func transformVersion(version string) string {
	if len(version) > 0 {
		switch version[0:0] {
		case "~":
			return version[1:]
		case "^":
			return version[1:]
		case ">":
			if version[0:1] == ">=" {
				return version[2:]
			}
			return version[1:]
		case "<":
			if version[0:1] == "<=" {
				return version[2:]
			}
			return version[1:]
		case "=":
			return version[1:]
		}
		return version
	}
	return version
}

func WriteBrevFile(t *terminal.Terminal, deps []string, gitURL string, path string) error {
	fmt.Println("\n\nGitUrl: ", gitURL)
	fmt.Println("** Found Dependencies **")
	t.Vprint(t.Yellow(strings.Join(deps, " \n")))
	shellString := GenerateShellScript(path)
	fmt.Println(GenerateLogs(shellString))
	mderr := os.MkdirAll(filepath.Join(path, ".brev"), os.ModePerm)
	if mderr == nil {
		// generate a string that is the collections.Concatenation of dependency-ordering the contents of all the dependencies
		// found by cat'ing the directory generated from the deps string, using the translated ruby code with go generics

		// place within .brev and write setup.sh with the result of this
		f, err := os.Create(filepath.Join(path, ".brev", "setup.sh")) //nolint:gosec // todo
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		defer f.Close() //nolint:errcheck // todo
		_, err = f.WriteString(shellString)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = f.Sync()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		return breverrors.WrapAndTrace(mderr)
	}
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

	if len(parts) == 2 { //nolint:gocritic // todo
		return ShellFragment{Name: &parts[0], Tag: &parts[1]}
	} else if len(parts) == 1 {
		return ShellFragment{Name: &parts[0]}
	} else {
		return ShellFragment{Comment: &commentFreeLine}
	}
}

func toDefsDict(shFragment []ShellFragment) map[string]ShellFragment {
	return collections.Foldl(func(acc map[string]ShellFragment, frag ShellFragment) map[string]ShellFragment {
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
	acc := collections.Foldl(func(acc scriptAccumulator, line string) scriptAccumulator {
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
	path := filepath.Join(collections.Concat([]string{"templates"}, subPaths)...)
	script, err := templateFs.Open(path)
	if err != nil && !noversion {
		if subPaths[0] != subPaths[1] {
			path = filepath.Join(collections.Concat([]string{"templates"}, collections.Duplicate(subPaths[0]))...)
			script, err = templateFs.Open(path)
			if err != nil {
				return []ShellFragment{}, breverrors.WrapAndTrace(err)
			}
		} else {
			return []ShellFragment{}, errors.New(strings.Join([]string{"Path does not exist:", path}, " "))
		}
	}
	out, err := ioutil.ReadAll(script)
	stringScript := string(out)
	if !noversion {
		stringScript = strings.ReplaceAll(stringScript, "${version}", subPaths[1])
	}
	// fmt.Println(stringScript)
	if err != nil {
		return []ShellFragment{}, breverrors.WrapAndTrace(err)
	}
	return fromSh(stringScript), nil
}

func toSh(script []ShellFragment) string {
	// collections.Flatmap across generating the script from all of the component shell bits
	return strings.Join(collections.Flatmap(func(frag ShellFragment) []string {
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
		if len(dependencies) > 0 {
			returnval = append(returnval, strings.Join([]string{"# dependencies:", strings.Join(dependencies, " ")}, " "))
		}
		if comment != nil {
			returnval = append(returnval, strings.Join([]string{"#", *comment}, " "))
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
	return collections.Flatmap(func(dep_name string) []string {
		return append(findDependencies(dep_name, baselineDependencies, globalDependencies), dep_name)
	}, dependencies)
}

func splitIntoLibraryAndSeq(shFragments []ShellFragment) ([]string, map[string]ShellFragment, []string) {
	return collections.Fmap(func(some ShellFragment) string {
		if some.Name != nil {
			return *some.Name
		} else {
			return ""
		}
	}, shFragments), toDefsDict(shFragments), []string{}
}

func prependDependencies(shName string, baselineDependencies map[string]ShellFragment, globalDependencies map[string]ShellFragment) OrderDefsFailures {
	dependencies := collections.Uniq(findDependencies(shName, baselineDependencies, globalDependencies))
	// baseline_deps := collections.Filter(func (dep string) bool {
	// 	if _, ok := baselineDependencies[dep]; ok {
	// 		return true
	// 	}
	// 	return false
	// }, dependencies)
	nonBaselineDependencies := collections.Filter(dependencies, func(dep string) bool {
		if _, ok := baselineDependencies[dep]; ok {
			return false
		}
		return true
	})
	canBeFixedDependencies := collections.Filter(nonBaselineDependencies, func(dep string) bool {
		if _, ok := globalDependencies[dep]; ok {
			return true
		}
		return false
	})
	cantBeFixedDependencies := collections.Difference(nonBaselineDependencies, canBeFixedDependencies)

	addedBaselineDependencies := collections.Foldl(
		func(deps map[string]ShellFragment, dep string) map[string]ShellFragment {
			deps[dep] = globalDependencies[dep]
			return deps
		}, map[string]ShellFragment{}, canBeFixedDependencies)
	return OrderDefsFailures{Order: append(dependencies, shName), Defs: collections.DictMerge(addedBaselineDependencies, baselineDependencies), Failures: cantBeFixedDependencies}
}

type OrderDefsFailures struct {
	Order    []string
	Defs     map[string]ShellFragment
	Failures []string
}

func addDependencies(shFragments []ShellFragment, _ map[string]ShellFragment, globalDependencies map[string]ShellFragment) OrderDefsFailures {
	//     ## it's a left fold across the importFile statements
	//     ## at any point, if the dependencies aren't already in the loaded dictionary
	//     ## we add them before we add the current item, and we add them and the current item into the loaded dictionary
	order, shellDefs, failures := splitIntoLibraryAndSeq(shFragments)
	newestOrderDefsFailures := collections.Foldl(func(acc OrderDefsFailures, shName string) OrderDefsFailures {
		newOrderDefsFailures := prependDependencies(shName, acc.Defs, globalDependencies)
		return OrderDefsFailures{Order: append(collections.Concat(acc.Order, newOrderDefsFailures.Order), shName), Failures: collections.Concat(acc.Failures, newOrderDefsFailures.Failures), Defs: collections.DictMerge(acc.Defs, newOrderDefsFailures.Defs)}
	}, OrderDefsFailures{Order: []string{}, Failures: []string{}, Defs: shellDefs}, order)
	return OrderDefsFailures{Order: collections.Uniq(newestOrderDefsFailures.Order), Defs: newestOrderDefsFailures.Defs, Failures: collections.Uniq(collections.Concat(failures, newestOrderDefsFailures.Failures))}
}

func process(shFragments []ShellFragment, baselineDependencies map[string]ShellFragment, globalDependencies map[string]ShellFragment) []ShellFragment {
	resultOrderDefsFailures := addDependencies(shFragments, baselineDependencies, globalDependencies)
	// TODO print or return the failed installation instructions "FAILED TO FIND INSTALLATION INSTRUCTIONS FOR: #{failures}" if failures.length > 0
	return collections.Fmap(func(x string) ShellFragment {
		return resultOrderDefsFailures.Defs[x]
	}, resultOrderDefsFailures.Order)
}

func MergeShells(deps ...string) string {
	importedDependencies := collections.Flatmap(func(dep string) []ShellFragment {
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
		jsonstring, _ := files.CatFile(paths[i])
		value := gjson.Get(jsonstring, keypath)
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

// #nosec G204

func readGoMod(filePath string) (string, error) {
	contents, err := files.CatFile(filePath)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	lines := strings.Split(contents, "\n")

	for _, line := range lines {
		if strings.Contains(line, "go ") {
			return strings.Split(line, " ")[1], nil
		}
	}

	return "", nil
}
