package test

import (
	"fmt"
	"os"
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

			TypeItToMe("\nPyEnvGPT is starting 🤙\n")
			fmt.Println("")

			str := t.Yellow("Detecting Operating System... ") + t.Green(getOperatingSystem()) + "\n"
			TypeItToMe(str)
			fmt.Println("")

			str = t.Yellow("Detecting Python2 Version... ") + t.Green(getPythonVersion()) + "\n"
			TypeItToMe(str)
			str = t.Yellow("Detecting Python3 Version... ") + t.Green(getPython3Version()) + "\n"
			TypeItToMe(str)
			fmt.Println("")

			str = t.Yellow("Detecting Virtual Environment... ") + t.Green(getEnvironmentDetails()) + "\n"
			TypeItToMe(str)
			fmt.Println("")

			var pythonVersion string
			prompt := &survey.Select{
				Message: "Which Python version would you like to install?:",
				Options: []string{"Python 2.7", "Python 3.6", "Python 3.7", "Python 3.8", "Python 3.9", "Python 3.10"},
			}
			survey.AskOne(prompt, &pythonVersion)

			fmt.Printf("You chose to install %s using OS package manager.\n", pythonVersion)
			// Based on the responses, you can now run the appropriate installation functions.

			err := installPythonWithPackageManager(pythonVersion)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func runCommandWithOutput(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installPythonWithVenv(version string) error {
	t := terminal.New()

	version = strings.Replace(version, "Python ", "", -1) // Replace "Python " with ""

	fmt.Printf("Checking for Python %s...\n", version)
	checkCmd := exec.Command("python"+version, "--version")

	// Try running python with the desired version
	if err := checkCmd.Run(); err != nil {
		fmt.Println(t.Red("Python %s is not installed.\n", version))
		fmt.Println(t.Red("Please install the desired version of Python and try again.\n"))
		return err
	}

	t.Printf("Python %s is installed. Creating a new virtual environment...\n", version)

	// Create a new virtual environment using Python's built-in venv module
	venvCmd := exec.Command("python"+version, "-m", "venv", "myenv")
	if err := runCommandWithOutput(venvCmd); err != nil {
		t.Printf(t.Red("Failed to create a new virtual environment: %s\n", err))
		return err
	}

	t.Printf("Created a new virtual environment at myenv.\n")
	t.Printf("You can activate it using the following command:\n")

	// Provide instructions for activating the virtual environment, which depends on the user's shell
	switch shell := runtime.GOOS; shell {
	case "windows":
		t.Printf("myenv\\Scripts\\activate\n")
	default:
		t.Printf("source myenv/bin/activate\n")
	}

	return nil
}

func getOperatingSystem() string {
	return runtime.GOOS
}

func getPythonVersion() string {
	cmd := exec.Command("python", "--version")
	output, err := cmd.CombinedOutput()
	pythonPath, _ := exec.LookPath("python")
	if err != nil {
		return "Python 2 not detected"
	}
	return strings.TrimSpace(string(output)) + "\n\tPath: " + pythonPath
}

func getPython3Version() string {
	cmd := exec.Command("python3", "--version")
	output, err := cmd.CombinedOutput()
	pythonPath, _ := exec.LookPath("python3")
	if err != nil {
		return "Python 3 not detected"
	}
	return strings.TrimSpace(string(output)) + "\n\tPath: " + pythonPath
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

func installPythonWithPackageManager(version string) error {
	t := terminal.New()

	version = strings.Replace(version, "Python ", "", -1) // Replace "Python " with ""

	fmt.Printf("Checking for Python %s...\n", version)
	checkCmd := exec.Command("python"+version, "--version")

	// Try running python with the desired version
	if err := checkCmd.Run(); err == nil {
		fmt.Println(t.Green("Python %s is already installed.\n", version))
		return nil
	}

	t.Printf("Python %s is not installed. Attempting to install...\n", version)

	// Install Python with the appropriate command for the user's operating system
	switch os := runtime.GOOS; os {
	case "windows":
		// On Windows, Python should be installed via the official installer or Microsoft Store
		fmt.Println(t.Red("Cannot automatically install Python on Windows. Please install it manually."))
		return fmt.Errorf("Cannot automatically install Python on Windows")
	case "darwin":
		// On macOS, Python can be installed via Homebrew
		installCmd := exec.Command("/bin/bash", "-c", "brew install python@"+version)
		if err := runCommandWithOutput(installCmd); err != nil {
			t.Printf(t.Red("Failed to install Python %s: %s\n", version, err))
			return err
		}
	case "linux":
		// On Linux, Python can be installed via apt (this may vary between distributions)
		installCmd := exec.Command("/bin/bash", "-c", "sudo apt-get install python"+version)
		if err := runCommandWithOutput(installCmd); err != nil {
			t.Printf(t.Red("Failed to install Python %s: %s\n", version, err))
			return err
		}
	default:
		// Unknown operating system
		fmt.Println(t.Red("Unsupported operating system for Python installation: %s", os))
		return fmt.Errorf("Unsupported operating system for Python installation: %s", os)
	}

	t.Printf("Python %s installation was successful.\n", version)

	return nil
}
