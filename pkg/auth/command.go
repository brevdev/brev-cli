package auth

import (
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/terminal"
)

func NewCmdLogin(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "login",
		Annotations: map[string]string{"housekeeping": ""},
		RunE: func(cmd *cobra.Command, args []string) error {
			return loginAndInitialize(t)
		},
	}
	return cmd
}
