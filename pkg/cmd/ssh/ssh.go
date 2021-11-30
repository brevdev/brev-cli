package ssh

import (
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
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
		// ValidArgs:   brevapi.GetWorkspaceNames(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			// _, err = brevapi.CheckOutsideBrevErrorMessage(t)
			return breverrors.WrapAndTrace(err)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{}, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveDefault
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := ssh(t, args[0])
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func ssh(_ *terminal.Terminal, _ string) error {
	// exec.Command("ssh " + workspace.DNS)
	return nil
}
