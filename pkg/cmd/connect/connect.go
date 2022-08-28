package connect

import (
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var (
	short   = "Connect Brev to your AWS account"
	long    = "Connect Brev to your AWS account"
	example = "brev connect"
)

type connectStore interface{}

func NewCmdConnect(t *terminal.Terminal, store connectStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "connect-aws",
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
	t.Vprintf("Connect the AWS IAM user to create dev environments in your AWS account.\n")
	t.Vprintf(t.Yellow("\tFollow the guide here: %s", "https://onboarding.brev.dev/connect-aws\n\n"))
	// t.Vprintf(t.Yellow("Connect the AWS IAM user to create dev environments in your AWS account.\n\n"))

	AccessKeyID := terminal.PromptGetInput(terminal.PromptContent{
		Label:    "Access Key ID: ",
		ErrorMsg: "error",
	})

	SecretAccessKey := terminal.PromptGetInput(terminal.PromptContent{
		Label:    "Secret Access Key: ",
		ErrorMsg: "error",
		Mask:     '*',
	})

	t.Vprintf("\n")
	t.Vprintf(AccessKeyID)
	t.Vprintf(SecretAccessKey)

	return nil
}
