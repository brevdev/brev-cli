// Package link is for the ssh command
package link

import (
	"os"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func getWorkspaces() []string {
	// // func getWorkspaces(orgID string) []brev_api.Workspace {
	activeorg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		return nil
		// return err
	}

	ws_cache, err := brev_api.Get_ws_cache_data()
	if err != nil {
		return nil
		// return err
	}
	for _, v := range ws_cache {
		if v.OrgID == activeorg.ID {
			var workspace_names []string
			for _, w := range v.Workspaces {
				workspace_names = append(workspace_names, w.Name)
			}
			return workspace_names
			// get a list of jsut the names
		}
	}
	return nil

	// client, _ := brev_api.NewClient()
	// // wss = workspaces, is that a bad name?
	// wss, _ := client.GetWorkspaces(activeorg.ID)

	// ws_names := []string{}
	// for _, v := range wss {
	// 	ws_names = append(ws_names, v.Name)
	// }

	// return ws_names
}

var sshLinkLong = "Enable a local ssh tunnel, setup private key auth, and give connection string"

var sshLinkExample = "brev link <ws_name>"

var (
	testCert = ``
	testKey  = ``
)

func NewCmdSSHLink(t *terminal.Terminal) *cobra.Command {
	host := "https://api.k8s.brevstack.com"
	k8sCert := []byte(testCert)
	k8sKeyFile := []byte(testKey)

	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	k8sClient, err := kubernetes.NewForConfig(&rest.Config{
		Host: host,
		TLSClientConfig: rest.TLSClientConfig{
			CertData: k8sCert,
			KeyData:  k8sKeyFile,
		},
	})
	if err != nil {
		panic(err)
	}

	opts := portforward.NewPortForwardOptions(
		k8sClient,
		&portforward.DefaultPortForwarder{
			IOStreams: streams,
		},
	)

	cmd := &cobra.Command{
		Use:                   "link",
		DisableFlagsInUseLine: true,
		Short:                 "Enable a local ssh link tunnel, setup private key auth, and give connection string",
		Long:                  sshLinkLong,
		Example:               sshLinkExample,
		Args:                  cobra.MinimumNArgs(1),
		ValidArgs:             getWorkspaces(),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.RunPortforward())
		},
	}
	return cmd
}
