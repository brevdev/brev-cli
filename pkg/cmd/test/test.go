package test

import (
	"fmt"
	"os"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
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
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
	server.RPCServerTaskStore
}

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdTest(_ *terminal.Terminal, store TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		// Args:                  cmderrors.TransformToValidationError(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			// fmt.Println("ayo")
			fmt.Println("reading file")
			res, err := GetOnboardingStep()
			if err != nil {
				return err
			}
			fmt.Println(res)

			newVal := 1
			fmt.Println("writing " + fmt.Sprint(newVal) + " to file")
			err = SetOnboardingStep(1)
			if err != nil {
				return err
			}

			res, err = GetOnboardingStep()
			if err != nil {
				return err
			}
			fmt.Println(res)

			return nil
		},
	}

	return cmd
}

func GetOnboardingFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	path, err := files.GetOnboardingStepPath(home)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	return path, nil
}

type OnboardingObject struct {
	Step int `json:"step"`
}

func GetOnboardingStep() (int, error) {
	// get path
	path, err := GetOnboardingFilePath()
	if err != nil {
		return 0, breverrors.WrapAndTrace(err)
	}

	// BANANA: ensure path exists
	// exists, err := afero.Exists(s.fs, path)
	// if err != nil {
	// 	return 0, breverrors.WrapAndTrace(err)
	// }
	// if !exists {
	// 	return nil, &breverrors.CredentialsFileNotFound{}
	// }

	// read file
	var oo OnboardingObject
	err = files.ReadJSON(files.AppFs, path, &oo)
	if err != nil {
		return 0, breverrors.WrapAndTrace(err)
	}

	// return data
	return oo.Step, nil
}

func SetOnboardingStep(step int) error {
	// get path
	path, err := GetOnboardingFilePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// BANANA: ensure path exists
	// exists, err := afero.Exists(s.fs, path)
	// if err != nil {
	// 	return 0, breverrors.WrapAndTrace(err)
	// }
	// if !exists {
	// 	return nil, &breverrors.CredentialsFileNotFound{}
	// }

	// write file
	oo := OnboardingObject{
		Step: step,
	}
	err = files.OverwriteJSON(files.AppFs, path, &oo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// return data
	return nil
}
