// Package link is for the ssh command
package link

import (
	"os"

	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var sshLinkLong = "Enable a local ssh tunnel, setup private key auth, and give connection string"

var sshLinkExample = "brev link <ws_name>"

type WorkspaceResolver interface {
	portforward.WorkspaceResolver

	GetWorkspaceNames() ([]string, error)
}

func NewCmdLink(workspaceResolver WorkspaceResolver, k8sClient k8s.K8sClient) *cobra.Command {
	opts := portforward.NewPortForwardOptions(
		k8sClient,
		workspaceResolver,
		&portforward.DefaultPortForwarder{
			IOStreams: genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		},
	)

	workspaceNames, err := workspaceResolver.GetWorkspaceNames()
	if err != nil {
		panic(err) // TODO
	}

	// link [resource id] -p 2222
	cmd := &cobra.Command{
		Use:                   "link",
		DisableFlagsInUseLine: true,
		Short:                 "Enable a local ssh link tunnel, setup private key auth, and give connection string",
		Long:                  sshLinkLong,
		Example:               sshLinkExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             workspaceNames,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.RunPortforward())
		},
	}
	return cmd
}
