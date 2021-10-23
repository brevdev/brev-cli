// Package cmd is the entrypoint to cli
package cmd

import (
	"io"
	"os"

	"github.com/brevdev/brev-cli/pkg/cmd/get"
	"github.com/brevdev/brev-cli/pkg/cmd/login"
	"github.com/brevdev/brev-cli/pkg/cmd/logout"
	"github.com/brevdev/brev-cli/pkg/cmd/ls"
	"github.com/brevdev/brev-cli/pkg/cmd/set"
	"github.com/brevdev/brev-cli/pkg/cmd/ssh"
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
		Use:   "brev-cli",
		Short: "brev client for managing workspaces",
		Long: `
      brev client for managing workspaces

      Find more information at:
            https://brev.dev`,
		Run: runHelp,
	}

	cmds.AddCommand(ssh.NewCmdSSH(t))
	cmds.AddCommand(login.NewCmdLogin())
	cmds.AddCommand(get.NewCmdGet(t))
	cmds.AddCommand(set.NewCmdSet(t))
	cmds.AddCommand(ls.NewCmdLs(t))
	cmds.AddCommand(logout.NewCmdLogout())

	return cmds
}

func runHelp(cmd *cobra.Command, _ []string) {
	cmd.Help()
}
