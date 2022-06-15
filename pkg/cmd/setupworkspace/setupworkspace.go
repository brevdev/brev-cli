package setupworkspace

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/setupworkspace"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/spf13/cobra"
)

type SetupWorkspaceStore interface {
	GetSetupParams() (*store.SetupParamsV0, error)
	WriteSetupScript(script string) error
	GetSetupScriptPath() string
	GetCurrentUser() (*entity.User, error)
	GetCurrentWorkspaceID() (string, error)
}

const Name = "setupworkspace"

// Internal command for setting up workspace // v1 similar to k8s post-start script
func NewCmdSetupWorkspace(store SetupWorkspaceStore) *cobra.Command {
	var forceEnableSetup bool
	cmd := &cobra.Command{
		Annotations: map[string]string{"hidden": ""},
		Use:         Name,
		RunE: func(cmd *cobra.Command, args []string) error {
			breverrors.GetDefaultErrorReporter().AddTag("command", Name)
			_, err := store.GetCurrentWorkspaceID() // do this to error reporting
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			fmt.Println("setting up workspace")

			params, err := store.GetSetupParams()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			if !featureflag.IsDev() {
				_, err = store.GetCurrentUser() // do this to set error user reporting
				if err != nil {
					fmt.Println(err)
					if !params.DisableSetup {
						breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err, "setup continued"))
					}
				}
			}

			if !forceEnableSetup && params.DisableSetup {
				fmt.Printf("WARNING: setup script not running [params.DisableSetup=%v, forceEnableSetup=%v]", params.DisableSetup, forceEnableSetup)
				return nil
			}

			err = setupworkspace.SetupWorkspace(params)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			fmt.Println("done setting up workspace")
			return nil
		},
	}
	cmd.PersistentFlags().BoolVar(&forceEnableSetup, "force-enable", false, "force the setup script to run despite params")

	return cmd
}
