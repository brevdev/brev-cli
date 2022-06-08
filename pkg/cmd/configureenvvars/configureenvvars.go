package configureenvvars

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
)

type ConfigureEnvVarsStore interface{}

func NewCmdConfigureEnvVars(t *terminal.Terminal, cevStore ConfigureEnvVarsStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "configure-env-vars",
		DisableFlagsInUseLine: true,
		Short:                 "configure env vars in supported shells",
		Long:                  "Import your IDE config",
		Example:               "",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunConfigureEnvVars(cevStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func RunConfigureEnvVars(cevStore ConfigureEnvVarsStore) error {
	bashConfigurer := BashConfigurer{}
	zshConfigurer := ZshConfigurer{}
	shellConfigurers := []ShellConfigurer{bashConfigurer, zshConfigurer}

	var allErr error
	for _, sc := range shellConfigurers {
		err := sc.Configure()
		if err != nil {
			allErr = multierror.Append(allErr, err)
		}
	}
	if allErr != nil {
		return breverrors.WrapAndTrace(allErr)
	}
	return nil
}

type ShellConfigurer interface {
	Configure() error
}

type BashConfigurer struct{}

var _ ShellConfigurer = BashConfigurer{}

func (b BashConfigurer) Configure() error {
	return nil
}

type ZshConfigurer struct{}

var _ ShellConfigurer = ZshConfigurer{}

func (b ZshConfigurer) Configure() error {
	return nil
}
