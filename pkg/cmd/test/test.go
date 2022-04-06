package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/server"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	startLong    = "[internal] test"
	startExample = "[internal] test"
)

type TestStore interface {
	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	CopyBin(targetBin string) error
	GetSetupScriptContentsByURL(url string) (string, error)
	server.RPCServerTaskStore
}

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdTest(t *terminal.Terminal, store TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		// Args:                  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// paths := recursivelyFindFile(t, "go.mod", "./")
			// for _, v := range paths {
			// 	fmt.Println(v)
			// }

			res := isNode(t, "./")
			fmt.Println(res)

			return nil
		},
	}

	return cmd
}

func isNode(t *terminal.Terminal, path string) bool {
	paths := recursivelyFindFile(t, []string{"package-lock.json", "package.json", "node_modules"}, path)

	if len(paths) > 0 {
		return true
	}
	return false
}

func isRust(t *terminal.Terminal, path string) bool {
	paths := recursivelyFindFile(t, []string{"Cargo.toml", "Cargo.lock"}, path)

	if len(paths) > 0 {
		return true
	}
	return false
}

func isGo(t *terminal.Terminal, path string) bool {
	paths := recursivelyFindFile(t, []string{"go.mod"}, path)

	if len(paths) > 0 {
		return true
	}
	return false
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

// Returns list of paths to file
func recursivelyFindFile(t *terminal.Terminal, filename_s []string, path string) []string {
	var paths []string

	files, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println(err)
	}

	for _, f := range files {
		dir, err := os.Stat(path + f.Name())
		if err != nil {
			fmt.Println(t.Red(err.Error()))
		} else {
			for _, filename := range filename_s {

				r, _ := regexp.Compile(filename)
				res := r.MatchString(f.Name())

				if res {
					t.Vprint(t.Yellow(filename) + "---" + t.Yellow(path+f.Name()))
					paths = append(paths, path+f.Name())
				}
			}

			if dir.IsDir() {
				paths = append(paths, recursivelyFindFile(t, filename_s, path+f.Name()+"/")...)
			}
		}
	}

	// TODO: make the list unique

	return paths
}
