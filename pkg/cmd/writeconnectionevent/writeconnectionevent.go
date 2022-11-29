package writeconnectionevent

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

type writeConnectionEventStore interface {
	WriteConnectionEvent() error
}

func NewCmdwriteConnectionEvent(t *terminal.Terminal, store writeConnectionEventStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "write-connection-event",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunWriteConnectionEvent(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func RunWriteConnectionEvent(_ *terminal.Terminal, _ []string, store writeConnectionEventStore) error {
	err := store.WriteConnectionEvent()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
