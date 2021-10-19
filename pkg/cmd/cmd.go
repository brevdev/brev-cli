// Package cmd is the entrypoint to cli
package cmd

import (
	"io"
	"os"

	"github.com/brevdev/brev-cli/pkg/cmd/ssh"
	"github.com/spf13/cobra"
)

func NewDefaultBrevCommand() *cobra.Command {
	cmd := NewBrevCommand(os.Stdin, os.Stdout, os.Stderr)
	return cmd
}

func NewBrevCommand(in io.Reader, out io.Writer, err io.Writer) *cobra.Command {
	cmds := &cobra.Command{
		Use:   "brev",
		Short: "brev client for managing workspaces",
		Long: `
      brev client for managing workspaces

      Find more information at:
            https://brev.dev`,
		Run: runHelp,
	}

	cmds.AddCommand(ssh.NewCmdSSH())

	return cmds
}

func runHelp(cmd *cobra.Command, _ []string) {
	cmd.Help()
}
