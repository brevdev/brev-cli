package postinstall

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

type postinstallStore interface{
	RegisterNotificationEmail(string ) error
}

func NewCmdpostinstall(t *terminal.Terminal, store postinstallStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "postinstall",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := Runpostinstall(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func Runpostinstall(
	t *terminal.Terminal,
	args []string,
	store postinstallStore,
) error {
	email := terminal.PromptGetInput(terminal.PromptContent{
		Label:    "Email: ",
		ErrorMsg: "error",
	})

	err := store.RegisterNotificationEmail(email)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
