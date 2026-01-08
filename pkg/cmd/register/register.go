// Package register provides the brev register command for DGX Spark registration
package register

import (
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	registerLong = `Register your DGX Spark with NVIDIA Brev

Join the waitlist to be among the first to register your DGX Spark 
for early access integration with Brev.`

	registerExample = `  brev register`
)

func NewCmdRegister(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "register",
		Aliases:               []string{"spark"},
		DisableFlagsInUseLine: true,
		Short:                 "Register your DGX Spark with Brev",
		Long:                  registerLong,
		Example:               registerExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			runRegister(t)
			return nil
		},
	}

	return cmd
}

func runRegister(t *terminal.Terminal) {
	t.Vprint("\n")
	t.Vprint(t.Green("Thanks so much for your interest in registering your DGX Spark with Brev!\n\n"))
	t.Vprint("To be on the waitlist for early access to this feature, please fill out this form:\n\n")
	t.Vprint(t.Yellow("  👉 https://forms.gle/RHCHGmZuiMQQ2faA6\n\n"))
	t.Vprint("We will reach out to the provided email with updates and instructions on how to register soon (:\n")
	t.Vprint("\n")
}

// TODO
// this should use spark/enroll.go
