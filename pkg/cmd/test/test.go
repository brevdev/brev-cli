package test

import (
	"fmt"
	"strings"

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
			
			term1 := "test/with/backslash"
			term2 := "test-no-backslash"

			fmt.Println(SlashToDash(term1))
			fmt.Println(SlashToDash(term2))

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

func SlashToDash(s string) string {

	splitBySlash := strings.Split(s, "/")

	concatenated := strings.Join(splitBySlash[:], "-")

	return concatenated
}
