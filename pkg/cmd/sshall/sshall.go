package sshall

import (
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type sshAllOptions struct{}

func NewCmdSSHAll() *cobra.Command {
	opts := sshAllOptions{}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "ssh-all",
		DisableFlagsInUseLine: true,
		Short:                 "run all workspace ssh connections",
		Long:                  "run ssh connections for all running workspaces",
		Example:               "brev ssh-all",
		Args:                  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate())
			cmdutil.CheckErr(opts.RunSSHAll())
		},
	}
	return cmd
}

func (s *sshAllOptions) Complete(cmd *cobra.Command, args []string) error {
	return nil
}

func (s *sshAllOptions) Validate() error {
	return nil
}

func (s *sshAllOptions) RunSSHAll() error {
	return nil
}
