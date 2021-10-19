// Package ssh is for the ssh command
package ssh

import (
	"fmt"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var sshLong = ""

var sshExample = ""

type SshOptions struct{}

func NewCmdSSH() *cobra.Command {
	opts := SshOptions{}

	cmd := &cobra.Command{
		Use:                   "ssh TYPE/NAME [options] [LOCAL_PORT:]REMOTE_PORT [...[LOCAL_PORT_N:]REMOTE_PORT_N]",
		DisableFlagsInUseLine: true,
		Short:                 "enable a local ssh tunnel",
		Long:                  sshLong,
		Example:               sshExample,
		// ValidArgsFunction:     util.ResourceNameCompletionFunc(f, "pod"),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate(cmd, args))
			cmdutil.CheckErr(opts.RunSSH(cmd, args))
		},
	}
	return cmd
}

func (o *SshOptions) Complete(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not implemented")
}

func (o *SshOptions) Validate(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not implemented")
}

func (o *SshOptions) RunSSH(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not implemented")
}
