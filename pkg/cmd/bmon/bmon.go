package bmon

import (
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

type bmonStore interface {
	autostartconf.AutoStartStore
}

func NewCmdbmon(t *terminal.Terminal, store bmonStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "bmon",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := Runbmon(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func Runbmon(_ *terminal.Terminal, _ []string, store bmonStore) error {
	bmonConfig := autostartconf.NewBrevMonConfigure(
		store,
		false,
		"10m",
	)
	err := bmonConfig.Install()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
