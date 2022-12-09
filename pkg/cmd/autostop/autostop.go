package autostop

import (
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

func NewCmdautostop(t *terminal.Terminal, store autostopStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "autostop",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := Runautostop(
				runAutostopArgs{
					t:     t,
					args:  args,
					store: store,
				},
			)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

type autostopStore interface {
	AutoStopWorkspace(workspaceID string) (*entity.Workspace, error)
}

type runAutostopArgs struct {
	t     *terminal.Terminal
	args  []string
	store autostopStore
}

func Runautostop(args runAutostopArgs) error {
	_, err := args.store.AutoStopWorkspace(args.args[0])
	return breverrors.WrapAndTrace(err)
}
