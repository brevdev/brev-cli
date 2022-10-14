package savemoney

import (
	"fmt"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

type saveMoneyStore interface{}

func NewCmdSaveMoney(t *terminal.Terminal, store saveMoneyStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "save-money",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := SaveMoney(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func SaveMoney(t *terminal.Terminal, args []string, store saveMoneyStore) error {
	fmt.Println("Save monay")
	fmt.Println(args)
	return nil
}
