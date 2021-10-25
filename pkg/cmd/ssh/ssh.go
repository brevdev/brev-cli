// Package ssh is for the ssh command
package ssh

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func getWorkspaces() []string {
	// func getWorkspaces(orgID string) []brev_api.Workspace {
	activeorg, err := brev_api.GetActiveOrgContext()

	if err != nil {
		return nil
		// return err
	}

	client, _ := brev_api.NewClient()
	// wss = workspaces, is that a bad name?
	wss, _ := client.GetWorkspaces(activeorg.ID)

	ws_names := []string{}
	for _, v := range wss {
		ws_names = append(ws_names, v.Name)
	}

	return ws_names
}

var sshLong = "Enable a local ssh tunnel, setup private key auth, and give connection string"

var sshExample = "brev ssh <ws_name>"

type SSHOptions struct {
	Namespace string
	PodName   string
	Port      string
	KeyFile   string
	CertFile  string
}

func NewCmdSSH(t *terminal.Terminal) *cobra.Command {
	opts := SSHOptions{}

	cmd := &cobra.Command{
		Use:                   "ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Enable a local ssh tunnel, setup private key auth, and give connection string",
		Long:                  sshLong,
		Example:               sshExample,
		Args:                  cobra.MinimumNArgs(1),
		ValidArgs:             getWorkspaces(),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate())
			cmdutil.CheckErr(opts.RunSSH())
		},
	}
	return cmd
}

func (o *SSHOptions) Complete(cmd *cobra.Command, args []string) error {
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

func (o *SSHOptions) Validate() error {
	return nil
}

func (o *SSHOptions) RunSSH() error {
	portforward.RunPortForwardFromCommand(
		o.Namespace,
		o.PodName,
		[]string{o.Port},
	)
	return nil
}
