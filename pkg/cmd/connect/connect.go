package connect

import (
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

type connectStore interface{}

func NewCmdConnect(t *terminal.Terminal, store connectStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "connect",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunConnect(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func RunConnect(t *terminal.Terminal, args []string, store connectStore) error {
	return nil
}
