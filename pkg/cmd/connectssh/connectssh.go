// Package connectssh is for the ssh command
package connectssh

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var sshLong = ""

var sshExample = "Enable a local ssh tunnel, setup private key auth, and give connection string"

type ConnectSSHOptions struct {
	Namespace string
	PodName   string
	Port      string
	KeyFile   string
	CertFile  string
}

func NewCmdConnectSSH(t *terminal.Terminal) *cobra.Command {
	opts := ConnectSSHOptions{}

	cmd := &cobra.Command{
		Use:                   "connect-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Enable a local ssh tunnel, setup private key auth, and give connection string",
		Long:                  sshLong,
		Example:               sshExample,
		Args:                  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate())
			cmdutil.CheckErr(opts.RunConnectSSH())
		},
	}
	return cmd
}

func (o *ConnectSSHOptions) Complete(cmd *cobra.Command, args []string) error {
	for _, arg := range args {
		fmt.Println(arg)
	}

	o.Namespace = ""
	o.PodName = ""
	o.Port = "22:22"
	o.KeyFile = ""
	o.CertFile = ""

	return nil
}

func (o *ConnectSSHOptions) Validate() error {
	return nil
}

func (o *ConnectSSHOptions) RunConnectSSH() error {
	portforward.RunPortForwardFromCommand(
		o.Namespace,
		o.PodName,
		[]string{o.Port},
		o.KeyFile,
		o.CertFile,
	)
	return nil
}
