// Package cmd is the entrypoint to cli
package cmd

import (
	"io"
	"os"

	"github.com/brevdev/brev-cli/pkg/cmd/connectssh"
	"github.com/brevdev/brev-cli/pkg/cmd/login"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

func NewDefaultBrevCommand() *cobra.Command {
	cmd := NewBrevCommand(os.Stdin, os.Stdout, os.Stderr)
	return cmd
}

func NewBrevCommand(in io.Reader, out io.Writer, err io.Writer) *cobra.Command {
	t := terminal.New()

	cmds := &cobra.Command{
		Use:   "brev",
		Short: "brev client for managing workspaces",
		Long: `
      brev client for managing workspaces

      Find more information at:
            https://brev.dev`,
		Run: runHelp,
	}

	cmds.AddCommand(connectssh.NewCmdConnectSSH(t))
	cmds.AddCommand(login.NewCmdLogin(t))

	return cmds
}

func runHelp(cmd *cobra.Command, _ []string) {
	cmd.Help()
}
