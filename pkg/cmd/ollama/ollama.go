// Package ollama is for starting AI/ML model workspaces
package ollama

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	uutil "github.com/brevdev/brev-cli/pkg/util"
	"github.com/briandowns/spinner"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"github.com/spf13/cobra"
)

var (
	ollamaLong    = "Start an AI/ML model workspace with specified model types"
	ollamaExample = `
  brev ollama --model llama2
  brev ollama --model mistral7b
	`
	modelTypes = []string{"llama2", "llama3", "mistral7b"}
)

type OllamaStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
}

func validateModelType(modelType string) bool {
	for _, v := range modelTypes {
		if modelType == v {
			return true
		}
	}
	return false
}

func NewCmdOllama(t *terminal.Terminal, ollamaStore OllamaStore) *cobra.Command {
	var model string

	cmd := &cobra.Command{
		Use:                   "ollama",
		DisableFlagsInUseLine: true,
		Short:                 "Start an AI/ML model workspace",
		Long:                  ollamaLong,
		Example:               ollamaExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			if model == "" {
				return fmt.Errorf("model type must be specified")
			}

			isValid := validateModelType(model)
			if !isValid {
				return fmt.Errorf("invalid model type: %s", model)
			}

			err := runOllamaWorkspace(t, model, ollamaStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", "", "AI/ML model type (e.g., llama2, llama3, mistral7b)")
	return cmd
}

func runOllamaWorkspace(t *terminal.Terminal, model string, ollamaStore OllamaStore) error {
	_, err := ollamaStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	org, err := ollamaStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// // Placeholder for instance type, to be updated later
	instanceType := "A100-Placeholder"

	cwOptions := store.NewCreateWorkspacesOptions("default-cluster", model).WithInstanceType(instanceType)

	t.Vprintf("Creating AI/ML workspace %s with model %s in org %s\n", t.Green(cwOptions.Name), t.Green(model), t.Green(org.ID))
	s := t.NewSpinner()
	s.Suffix = " Creating your workspace. Hang tight 🤙"
	s.Start()
	dev := true
	var w *entity.Workspace
	if dev {
		time.Sleep(3 * time.Second)
	} else {
		w, err = ollamaStore.CreateWorkspace(org.ID, cwOptions)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	s.Stop()

	fmt.Print("\n")
	t.Vprint(t.Green("Your AI/ML workspace is ready!\n"))
	displayConnectBreadCrumb(t, w)

	return nil
}

func displayConnectBreadCrumb(t *terminal.Terminal, workspace *entity.Workspace) {
	t.Vprintf(t.Green("Connect to the workspace:\n"))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev open %s\t# brev open <NAME> -> open workspace in VS Code\n", workspace.Name)))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev shell %s\t# brev shell <NAME> -> ssh into workspace (shortcut)\n", workspace.Name)))
}

func startWorkspaceIfStopped(t *terminal.Terminal, tstore OllamaStore, wsIDOrName string, workspace *entity.Workspace) error {
	startedWorkspace, err := tstore.StartWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprintf(t.Yellow("Instance %s is starting. \n\n", startedWorkspace.Name))
	workspace, err = util.GetUserWorkspaceByNameOrIDErr(tstore, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func tryToInstallExtensions(
	t *terminal.Terminal,
	extIDs []string,
) {
	for _, extID := range extIDs {
		extInstalled, err0 := uutil.IsVSCodeExtensionInstalled(extID)
		if !extInstalled {
			err1 := uutil.InstallVscodeExtension(extID)
			isRemoteInstalled, err2 := uutil.IsVSCodeExtensionInstalled(extID)
			if !isRemoteInstalled {
				err := multierror.Append(err0, err1, err2)
				t.Print(t.Red("Couldn't install the necessary VSCode extension automatically.\nError: " + err.Error()))
				t.Print("\tPlease install VSCode and the following VSCode extension: " + t.Yellow(extID) + ".\n")
				_ = terminal.PromptGetInput(terminal.PromptContent{
					Label:      "Hit enter when finished:",
					ErrorMsg:   "error",
					AllowEmpty: true,
				})
			}
		}
	}
}

func waitForSSHToBeAvailable(t *terminal.Terminal, s *spinner.Spinner, sshAlias string) error {
	counter := 0
	for {
		cmd := exec.Command("ssh", "-o", "ConnectTimeout=1", sshAlias, "echo", " ")
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}

		outputStr := string(out)
		stdErr := strings.Split(outputStr, "\n")[1]

		if counter == 160 || !store.SatisfactorySSHErrMessage(stdErr) {
			return breverrors.WrapAndTrace(errors.New("\n" + stdErr))
		}

		if counter == 10 {
			s.Suffix = t.Green(" waiting for SSH connection to be available 🤙")
		}

		counter++
		time.Sleep(1 * time.Second)
	}
}

type vscodePathStore interface {
	GetWindowsDir() (string, error)
}

func getWindowsVsCodePaths(store vscodePathStore) []string {
	wd, _ := store.GetWindowsDir()
	paths := append([]string{}, fmt.Sprintf("%s/AppData/Local/Programs/Microsoft VS Code/Code.exe", wd), fmt.Sprintf("%s/AppData/Local/Programs/Microsoft VS Code/bin/code", wd))
	return paths
}
