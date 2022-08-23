package scale

import (
	"fmt"

	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type scaleStore interface{}

func NewCmdscale(t *terminal.Terminal, store scaleStore) *cobra.Command {
	// string flag for class type

	cmd := &cobra.Command{
		Use:                   "scale",
		DisableFlagsInUseLine: true,
		Short:                 "TODO",
		Long:                  "TODO",
		Example:               "TODO",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := Runscale(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func Runscale(_ *terminal.Terminal, args []string, _ scaleStore) error {
	fmt.Println(args)
	return nil
}
