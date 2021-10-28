// Package link is for the ssh command
package link

import (
	"os"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var sshLinkLong = "Enable a local ssh tunnel, setup private key auth, and give connection string"

var sshLinkExample = "brev link <ws_name>"

func NewCmdLink(t *terminal.Terminal) *cobra.Command {
	// link [resource id] -p 2222
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "link",
		DisableFlagsInUseLine: true,
		Short:                 "Enable a local ssh link tunnel",
		Long:                  sshLinkLong,
		Example:               sshLinkExample,
		Args:                  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			k8sClientConfig, err := NewRemoteK8sClientConfig()
			if err != nil {
				t.Errprint(err, "")
			}
			k8sClient := k8s.NewDefaultClient(k8sClientConfig)

			opts := portforward.NewPortForwardOptions(
				k8sClient,
				WorkspaceResolver{},
				&portforward.DefaultPortForwarder{
					IOStreams: genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
				},
			)
			err = files.WriteSSHPrivateKey(string(k8sClientConfig.GetKey()))
			if err != nil {
				panic(err)
			}
			t.Printf("SSH Private Key:\n%s\n", files.GetSSHPrivateKeyFilePath())
			t.Printf("Connect to workspace: \n\nssh -p 2222   -i \"%s\" brev@0.0.0.0\n\n", files.GetSSHPrivateKeyFilePath())
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.RunPortforward())
		},
	}
	return cmd
}

type WorkspaceResolver struct{}

func (d WorkspaceResolver) GetWorkspaceByID(id string) (*brev_api.WorkspaceMetaData, error) {
	c, err := brev_api.NewClient()
	if err != nil {
		return nil, err
	}
	w, err := c.GetWorkspaceMetaData(id)
	if err != nil {
		return nil, err
	}

	return w, nil
}

type K8sClientConfig struct {
	host string
	cert []byte
	key  []byte
	ca   []byte
}

func NewRemoteK8sClientConfig() (*K8sClientConfig, error) {
	c, err := brev_api.NewClient()
	if err != nil {
		return nil, err
	}

	keys, err := c.GetMeKeys()
	if err != nil {
		return nil, err
	}

	clusterID := "k8s.brevstack.com"

	cluserKeys, err := keys.GetWorkspaceGroupKeysByGroupID(clusterID)
	if err != nil {
		return nil, err
	}

	return &K8sClientConfig{
		host: "https://api.k8s.brevstack.com",
		cert: []byte(cluserKeys.Cert),
		key:  []byte(keys.PrivateKey),
		ca:   []byte(cluserKeys.CA),
	}, nil
}

func (k K8sClientConfig) GetHost() string {
	return k.host
}

func (k K8sClientConfig) GetCert() []byte {
	return k.cert
}

func (k K8sClientConfig) GetKey() []byte {
	return k.key
}

func (k K8sClientConfig) GetCA() []byte {
	return k.ca
}
