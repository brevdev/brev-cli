package importpkg

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/setupscript"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"

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

	// Check for Rust Cargo Package
	deps, err := importStore.GetDependenciesForImport(path)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// if len(deps.Rust) > 0 {
	// 	t.Vprint(t.Green("\nRust detected!"))
	// 	t.Vprint(t.Yellow("{note}-- we still don't have Rust version support, so it always installs latest..."))
	// 	res := terminal.PromptGetInput(terminal.PromptContent{
	// 		Label:      "Click enter to install latest, or type preferred version:",
	// 		ErrorMsg:   "error",
	// 		AllowEmpty: true,
	// 	})
	// 	if len(res)>0 {
	// 		deps.Rust = res
	// 		t.Vprintf("Installing Rust v%s", deps.Rust)
	// 	} else {
	// 		t.Vprint("Installing Rust-- latest version.\n")
	// 	}
	// }
	// if len(deps.Node) > 0 {
	// 	t.Vprint("Install Node.\n")
	// }
	// if len(deps.Java) > 0 {
	// 	t.Vprint("Install Java.\n")
	// }
	// if len(deps.TS) > 0 {
	// 	t.Vprint("Install TS.\n")
	// }
	if len(deps.Go) > 0 {
		t.Vprint(t.Green("\nGolang detected!"))
		res := terminal.PromptGetInput(terminal.PromptContent{
			Label:      "Click enter to install Go v" + deps.Go + ", or type preferred version:",
			ErrorMsg:   "error",
			AllowEmpty: true,
		})
		if len(res) > 0 {
			deps.Go = res
			t.Vprintf("Installing Go v%s\n", deps.Go)
		} else {
			t.Vprintf("Installing Go v%s.\n", deps.Go)
		}
	}
	// if len(deps.Solana) > 0 {
	// 	t.Vprint("Install Solana.\n")
	// }

	err = clone(t, gitURL, orgflag, importStore, name, deps, detached)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

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

	fmt.Println("BANANA: remove this")
	fmt.Println(options.StartupScript)

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
