package test

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

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
			fmt.Println("Operating System: " + getOperatingSystem())
			fmt.Println(getPythonVersion())
			fmt.Println(getPython3Version())
			fmt.Println(getEnvironmentDetails())

			// t := terminal.New()
			// s := t.NewSpinner()

			// hello.TypeItToMe("PyEnvGPT is starting 🤙\n")
			// fmt.Println("")
			// hello.TypeItToMe("Detecting Operating System")
			// s.Start()
			// s.Suffix = " Detecting Operating System"
			// time.Sleep(1 * time.Second)
			// s.Stop()

			// res := util.DoesPathExist("/Users/naderkhalil/brev-cli")
			// fmt.Println(res)

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
		return "Python: Not detected"
	}
	return "Python: " + strings.TrimSpace(string(output))
}

func getPython3Version() string {
	cmd := exec.Command("python3", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "Python3: Not detected"
	}
	return "Python3: " + strings.TrimSpace(string(output))
}

func getEnvironmentDetails() string {
	condaCmd := exec.Command("bash", "-c", "source activate; conda env list")
	condaOutput, err := condaCmd.CombinedOutput()
	if err == nil && strings.Contains(string(condaOutput), "*") {
		return "Virtual Environment: Conda is being used"
	}

	venv := exec.Command("bash", "-c", "if [ -z \"$VIRTUAL_ENV\" ]; then echo 'No'; else echo 'Yes'; fi")
	venvOutput, err := venv.CombinedOutput()
	if err == nil && strings.Contains(string(venvOutput), "Yes") {
		return "Virtual Environment: venv is being used"
	}

	return "Virtual Environment: None detected"
}
