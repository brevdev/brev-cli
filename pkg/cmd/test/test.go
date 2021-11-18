package test

import (
	"os/exec"

	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	startLong    = "[internal] test"
	startExample = "[internal] test"
)

func NewCmdTest(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		Run: func(cmd *cobra.Command, args []string) {
			

			cmdd := exec.Command("which code")
			_, err := cmdd.Output()

			if err != nil {
				t.Vprintf("Please install the following VS Code extension: ms-vscode-remote.remote-ssh\n")

			} else {
				install := exec.Command("code --install-extension ms-vscode-remote.remote-ssh\n")
				_, err := install.Output()
				if err != nil {
					t.Vprintf("Please install the following VS Code extension: ms-vscode-remote.remote-ssh\n")
				}
			}



		},
	}

	return cmd
}
