// Package link is for the ssh command
package link

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/spf13/cobra"
)

var (
	Port           string
	sshLinkLong    = "Enable a local ssh tunnel, setup private key auth, and give connection string"
	sshLinkExample = "brev link <ws_name>"
)

func getWorkspaceNames() []string {
	activeOrg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		return nil
	}

	client, err := brev_api.NewCommandClient()
	if err != nil {
		return nil
	}
	wss, err := client.GetMyWorkspaces(activeOrg.ID)
	if err != nil {
		return nil
	}

	var wsNames []string
	for _, w := range wss {
		wsNames = append(wsNames, w.Name)
	}

	return wsNames
}

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
		ValidArgs:             getWorkspaceNames(),
		Run: func(cmd *cobra.Command, args []string) {
			t.Printf("Starting ssh link...\n")
			k8sClientConfig, err := NewRemoteK8sClientConfig()
			if err != nil {
				switch err.(type) {
				case *url.Error:
					t.Errprint(err, "\n\ncheck your internet connection")
					return

				default:
					t.Errprint(err, "")
				}
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
				t.Errprint(err, "")
			}
			sshPrivateKeyFilePath := files.GetSSHPrivateKeyFilePath()
			if Port == "" {
				Port = "2222:22"
			}
			cmdutil.CheckErr(opts.Complete(cmd, t, args, Port))
			t.Printf("SSH Private Key: %s\n", sshPrivateKeyFilePath)
			t.Printf(t.Green("\n\t1. Add SSH Key:\n"))
			t.Printf(t.Yellow("\t\tssh-add %s\n", sshPrivateKeyFilePath))
			t.Printf(t.Green("\t2. Connect to workspace:\n"))
			localPort := strings.Split(Port, ":")[0]
			t.Printf(t.Yellow("\t\tssh -p %s brev@0.0.0.0\n\n", localPort))
			cmdutil.CheckErr(opts.RunPortforward())
		},
	}
	cmd.Flags().StringVarP(&Port, "port", "p", "", "port forward flag describe me better")

	return cmd
}

type WorkspaceResolver struct{}

func (d WorkspaceResolver) GetWorkspaceByID(id string) (*brev_api.AllWorkspaceData, error) {
	c, err := brev_api.NewCommandClient()
	if err != nil {
		return nil, err
	}
	wmeta, err := c.GetWorkspaceMetaData(id)
	if err != nil {
		return nil, err
	}
	w, err := c.GetWorkspace(id)
	if err != nil {
		return nil, err
	}

	return &brev_api.AllWorkspaceData{WorkspaceMetaData: *wmeta, Workspace: *w}, nil
}

// This function will be long and messy, it's entirely built to check random error cases
// func GetWorkspaceByName(name string) (*brev_api.AllWorkspaceData, error) {
func (d WorkspaceResolver) GetWorkspaceByName(name string) (*brev_api.AllWorkspaceData, error) {
	c, err := brev_api.NewCommandClient()
	if err != nil {
		return nil, err
	}

	// Check ActiveOrg's workspaces before checking every orgs workspaces as fallback
	activeorg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		// TODO: we sould just check all possible workspaces here
		return nil, errors.New("Please set your active org or link to a workspace by it's ID")
	} else {
		workspaces, err := c.GetMyWorkspaces(activeorg.ID)
		if err != nil {
			return nil, err
		}
		for _, w := range workspaces {
			if w.Name == name {
				wmeta, err := c.GetWorkspaceMetaData(w.ID)
				if err != nil {
					return nil, err
				}
				return &brev_api.AllWorkspaceData{WorkspaceMetaData: *wmeta, Workspace: w}, nil
			}
		}
		// if there wasn't a workspace in the org, check all the orgs
	}

	orgs, err := c.GetOrgs()
	if err != nil {
		return nil, err
	}

	for _, o := range orgs {
		workspaces, err := c.GetWorkspaces(o.ID)
		if err != nil {
			return nil, err
		}

		for _, w := range workspaces {
			if w.Name == name {
				// Assemble full object
				wmeta, err := c.GetWorkspaceMetaData(w.ID)
				if err != nil {
					return nil, err
				}
				return &brev_api.AllWorkspaceData{WorkspaceMetaData: *wmeta, Workspace: w}, nil
			}
		}
	}

	return nil, fmt.Errorf("workspace with name %s does not exist", name)
}

type K8sClientConfig struct {
	host string
	cert []byte
	key  []byte
	ca   []byte
}

func NewRemoteK8sClientConfig() (*K8sClientConfig, error) {
	c, err := brev_api.NewCommandClient()
	if err != nil {
		return nil, err
	}

	keys, err := c.GetMeKeys()
	if err != nil {
		return nil, err
	}

	clusterID := config.GlobalConfig.GetDefaultClusterID()

	cluserKeys, err := keys.GetWorkspaceGroupKeysByGroupID(clusterID)
	if err != nil {
		return nil, err
	}

	return &K8sClientConfig{
		host: config.GlobalConfig.GetKubeAPIURL(),
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
