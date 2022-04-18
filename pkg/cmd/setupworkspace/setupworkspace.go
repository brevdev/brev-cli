package setupworkspace

import (
	"fmt"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/setupworkspace"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/spf13/cobra"
)

type SetupWorkspaceStore interface {
	GetSetupParams() (*store.SetupParamsV0, error)
	WriteSetupScript(script string) error
	GetSetupScriptPath() string
}

// Internal command for setting up workspace // v1 similar to k8s post-start script
func NewCmdSetupWorkspace(store SetupWorkspaceStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"hidden": ""},
		Use:         "setupworkspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("setting up workspace")
			params, err := store.GetSetupParams()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			err = setupworkspace.SetupWorkspace(params)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
	}

	return cmd
}

// Internal command for setting up workspace // to validate workspace works given params
func NewCmdValidateWorkspaceSetup(store SetupWorkspaceStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"hidden": ""},
		Use:         "validateworkspacesetup",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("validate workspace setup up")
			params, err := store.GetSetupParams()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			err = setupworkspace.ValidateSetup(*params)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func NewCmdTestWorkspaceSetup() *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"hidden": ""},
		Use:         "testworkspacesetup",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("test workspace setup up")
			// read in parameters
			// make and write post start script
			// exec post start script
		},
	}

	return cmd
}
