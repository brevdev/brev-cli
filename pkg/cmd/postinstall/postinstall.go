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

type postinstallStore interface {
	RegisterNotificationEmail(string) error
	WriteEmail(email string) error
}

func NewCmdpostinstall(_ *terminal.Terminal, store postinstallStore) *cobra.Command {
	// var email string

	cmd := &cobra.Command{
		Use: "postinstall",
		// DisableFlagsInUseLine: true,
		Short:   short,
		Long:    long,
		Example: example,
		RunE: func(cmd *cobra.Command, args []string) error {
			email := ""
			if len(args) > 0 {
				email = args[0]
			}
			err := Runpostinstall(store, email)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func Runpostinstall(store postinstallStore, email string) error {
	if email == "" {
		email = terminal.PromptGetInput(terminal.PromptContent{
			Label:    "Email: ",
			ErrorMsg: "error",
		})
	}

	err := store.WriteEmail(email)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = store.RegisterNotificationEmail(email)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
