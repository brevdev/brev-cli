package test

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
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
			var err error

			// SSH Keys
			brevapi.DisplayBrevLogo(t)
			t.Vprintf("\n")
			spinner := t.NewSpinner()
			spinner.Suffix = " fetching your public key"
			spinner.Start()
			err = brevapi.GetandDisplaySSHKeys(t)
			spinner.Stop()
			if err != nil {
				t.Vprintf(t.Red(err.Error()))
			}

			hasVSCode := brevapi.PromptSelectInput(brevapi.PromptSelectContent{
				Label:    "Do you use VS Code?",
				ErrorMsg: "error",
				Items:    []string{"yes", "no"},
			})
			if hasVSCode == "yes" {
				// TODO: check if user uses VSCode and intall extension for user
				t.Vprintf(t.Yellow("Please install the following VS Code extension: ms-vscode-remote.remote-ssh\n"))
			}

			// brevapi.InstallVSCodeExtension(t)

			// NOTE: this only works on Mac
			// err = beeep.Notify("Title", "Message body", "assets/information.png")
			// t.Vprintf("we just setn the beeeep")
			// if err != nil {
			// 	fmt.Println(err)
			// }

			// err = beeep.Beep(beeep.DefaultFreq, beeep.DefaultDuration)
			// if err != nil {
			// 	fmt.Println(err)
			// }

			// err = beeep.Alert("Title", "Message body", "assets/warning.png")
			// if err != nil {
			// 	fmt.Println(err)
			// }
		},
	}

	return cmd
}
