package setupworkspace

import (
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/setupworkspace"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"
)

type SetupWorkspaceStore interface {
	GetSetupParams() (*store.SetupParamsV0, error)
	WriteSetupScript(script string) error
	GetSetupScriptPath() string
	GetCurrentUser() (*entity.User, error)
}

const Name = "setupworkspace"

// Internal command for setting up workspace // v1 similar to k8s post-start script
func NewCmdSetupWorkspace(store SetupWorkspaceStore) *cobra.Command {
	var forceEnableSetup bool
	cmd := &cobra.Command{
		Annotations: map[string]string{"hidden": ""},
		Use:         Name,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("setting up workspace")
			if !featureflag.IsDev() {
				err := sentry.Init(sentry.ClientOptions{
					Dsn: config.GlobalConfig.GetSentryURL(),
				})
				if err != nil {
					fmt.Println(err)
				}

				user, err := store.GetCurrentUser()
				if err != nil {
					sentry.CaptureMessage(err.Error())
					sentry.CaptureException(err)
					return breverrors.WrapAndTrace(err)
				}

				scope := sentry.CurrentHub().Scope()
				scope.SetUser(sentry.User{
					ID:       user.ID,
					Username: user.Username,
					Email:    user.Email,
				})
				scope.SetTag("command", Name)
				defer sentry.Flush(2 * time.Second)
			}

			params, err := store.GetSetupParams()
			if err != nil {
				sentry.CaptureMessage(err.Error())
				sentry.CaptureException(err)
				return breverrors.WrapAndTrace(err)
			}

			if !forceEnableSetup && params.DisableSetup {
				fmt.Printf("WARNING: setup script not running [params.DisableSetup=%v, forceEnableSetup=%v]", params.DisableSetup, forceEnableSetup)
				return nil
			}

			err = setupworkspace.SetupWorkspace(params)
			if err != nil {
				sentry.CaptureMessage(err.Error())
				sentry.CaptureException(err)
				return breverrors.WrapAndTrace(err)
			}
			fmt.Println("done setting up workspace")

			return nil
		},
	}
	cmd.PersistentFlags().BoolVar(&forceEnableSetup, "force-enable", false, "force the setup script to run despite params")

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
