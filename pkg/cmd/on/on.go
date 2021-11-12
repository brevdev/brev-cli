package on

import (
	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/cmd/link"
	"github.com/brevdev/brev-cli/pkg/cmd/sshall"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type onOptions struct {
	on *On
}

func NewCmdOn() *cobra.Command {
	opts := onOptions{}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "on",
		DisableFlagsInUseLine: true,
		Short:                 "run all workspace ssh connections",
		Long:                  "run ssh connections for all running workspaces",
		Example:               "brev ssh-all",
		Args:                  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate())
			cmdutil.CheckErr(opts.RunOn())
		},
	}
	return cmd
}

func (s *onOptions) Complete(cmd *cobra.Command, args []string) error {
	k8sClientConfig, err := link.NewRemoteK8sClientConfig() // to resolve
	if err != nil {
		return err
	}
	_ = k8s.NewDefaultClient(k8sClientConfig) // to resolve

	// s.on = NewOn("", k8sClient)
	return nil
}

func (s onOptions) Validate() error {
	return nil
}

func (s onOptions) RunOn() error {
	return s.on.Run()
}

type SSHConfigurer interface {
	Config() error
	GetWorkspaces() ([]brev_api.WorkspaceWithMeta, error)
	GetConfiguredWorkspacePort(workspace brev_api.Workspace) (string, error)
}

type On struct {
	sshConfigurer SSHConfigurer
	k8sClient     k8s.K8sClient
}

func NewOn(sshConfigurer SSHConfigurer, k8sClient k8s.K8sClient) *On {
	return &On{
		sshConfigurer: sshConfigurer,
		k8sClient:     k8sClient,
	}
}

func (o On) Run() error {
	err := o.sshConfigurer.Config()
	if err != nil {
		return err
	}

	sshall := sshall.NewSSHAll(o.k8sClient, o.sshConfigurer)
	err = sshall.Run()
	if err != nil {
		return err
	}

	return nil
}
