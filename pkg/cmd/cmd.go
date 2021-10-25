// Package cmd is the entrypoint to cli
package cmd

import (
	"io"
	"os"

	"github.com/brevdev/brev-cli/pkg/cmd/get"
	"github.com/brevdev/brev-cli/pkg/cmd/link"
	"github.com/brevdev/brev-cli/pkg/cmd/login"
	"github.com/brevdev/brev-cli/pkg/cmd/logout"
	"github.com/brevdev/brev-cli/pkg/cmd/ls"
	"github.com/brevdev/brev-cli/pkg/cmd/set"
	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

func NewDefaultBrevCommand() *cobra.Command {
	cmd := NewBrevCommand(os.Stdin, os.Stdout, os.Stderr)
	return cmd
}

func NewBrevCommand(in io.Reader, out io.Writer, err io.Writer) *cobra.Command {
	t := terminal.New()
	var printVersion bool


	cmds := &cobra.Command{
		Use:   "brev-cli",
		Short: "brev client for managing workspaces",
		Long: `
      brev client for managing workspaces

      Find more information at:
            https://brev.dev`,
		Run: runHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			if printVersion {
				v, err := version.BuildVersionString(t)
				if err != nil {
					t.Errprint(err, "Failed to determine version")
					return err
				}
				t.Vprint(v)
				return nil
			} else {
				return cmd.Usage()
			}
		},
	}

	cmds.PersistentFlags().BoolVar(&printVersion, "version", false, "Print version output")

	createCmdTree(cmds, t)

	return cmds
}

func createCmdTree(cmd *cobra.Command, t *terminal.Terminal) {
	cmd.AddCommand(link.NewCmdSSHLink(t))
	cmd.AddCommand(login.NewCmdLogin())
	cmd.AddCommand(get.NewCmdGet(t))
	cmd.AddCommand(set.NewCmdSet(t))
	cmd.AddCommand(ls.NewCmdLs(t))
	cmd.AddCommand(logout.NewCmdLogout())
}

func runHelp(cmd *cobra.Command, _ []string) {
	cmd.Help()
}
