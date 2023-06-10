package test

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
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
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
}

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func TypeItToMe(s string) {
	sleepSpeed := 21

	sRunes := []rune(s)
	for i := 0; i < len(sRunes); i++ {
		time.Sleep(time.Duration(sleepSpeed) * time.Millisecond)
		fmt.Printf("%c", sRunes[i])
	}
}

func NewCmdTest(_ *terminal.Terminal, _ TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		// Args:                  cmderrors.TransformToValidationError(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			t := terminal.New()
			// s := t.NewSpinner()

			// TypeItToMe("PyEnvGPT is starting 🤙\n")
			TypeItToMe("\nPyEnvGPT is starting 🤙\n")
			fmt.Println("")

			// %%%%%%%%% Detect Operating System %%%%%%%%%
			str := t.Yellow("Detecting Operating System... ") + t.Green(getOperatingSystem()) + "\n"
			TypeItToMe(str)
			fmt.Println("")

			// %%%%%%%%% Detect Python Versions -- both of them %%%%%%%%%
			str = t.Yellow("Detecting Python2 Version... ") + t.Green(getPythonVersion()) + "\n"
			TypeItToMe(str)
			str = t.Yellow("Detecting Python3 Version... ") + t.Green(getPython3Version()) + "\n"
			TypeItToMe(str)
			fmt.Println("")

			// %%%%%%%%% Detect Virtual Environment %%%%%%%%%
			str = t.Yellow("Detecting Virtual Environment... ") + t.Green(getEnvironmentDetails()) + "\n"
			TypeItToMe(str)
			fmt.Println("")

			var pythonVersion string
			prompt := &survey.Select{
				Message: "Which Python version would you like to install?:",
				Options: []string{"Python 2.7", "Python 3.6", "Python 3.7", "Python 3.8", "Python 3.9"},
			}
			survey.AskOne(prompt, &pythonVersion)

			var installationMethod string
			prompt = &survey.Select{
				Message: "How would you like to install Python?:",
				Options: []string{"OS install", "virtualenv", "conda"},
			}
			survey.AskOne(prompt, &installationMethod)

			fmt.Printf("You chose to install %s using %s.\n", pythonVersion, installationMethod)
			// Based on the responses, you can now run the appropriate installation functions.

			return nil
		},
	}

	return cmd
}

func getOperatingSystem() string {
	return runtime.GOOS
}

func getPythonVersion() string {
	cmd := exec.Command("python", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "Not detected"
	}
	return strings.TrimSpace(string(output))
}

func getPython3Version() string {
	cmd := exec.Command("python3", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "Not detected"
	}
	return strings.TrimSpace(string(output))
}

func getEnvironmentDetails() string {
	condaCmd := exec.Command("bash", "-c", "source activate; conda env list")
	condaOutput, err := condaCmd.CombinedOutput()
	if err == nil && strings.Contains(string(condaOutput), "*") {
		return "Conda is being used"
	}

	venv := exec.Command("bash", "-c", "if [ -z \"$VIRTUAL_ENV\" ]; then echo 'No'; else echo 'Yes'; fi")
	venvOutput, err := venv.CombinedOutput()
	if err == nil && strings.Contains(string(venvOutput), "Yes") {
		return "venv is being used"
	}

	return "None detected"
}
