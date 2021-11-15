// Package get is for the get command
// TODO: delete this file if getmeprivatekeys isn't needed
package ssh

import (
	"os/exec"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

func NewCmdSSH(t *terminal.Terminal) *cobra.Command {
	// opts := SshOptions{}

	cmd := &cobra.Command{
		Use:         "ssh",
		Annotations: map[string]string{"ssh": ""},
		Short:       "SSH into your workspace",
		Long:        "SSH into your workspace",
		Example:     `brev ssh [workspace_name]`,
		Args:        cobra.ExactArgs(1),
		ValidArgs:   brevapi.GetWorkspaceNames(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}

			// _, err = brevapi.CheckOutsideBrevErrorMessage(t)
			return err
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{}, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveDefault
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ssh(t, args[0])
			return nil
		},
	}

	return cmd
}

func ssh(t *terminal.Terminal, wsname string) error {
	workspace, err := brevapi.GetWorkspaceFromName(wsname)
	if err != nil {
		return err
	}

	t.Vprint(workspace.DNS)
	exec.Command("ssh " + workspace.DNS)
	return nil
}
