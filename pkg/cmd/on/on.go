package on

import (
	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/cmd/sshall"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type onOptions struct{}

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
	return nil
}

func (s onOptions) Validate() error {
	return nil
}

func (s onOptions) RunOn() error {
	return nil
}

type SSHConfigurer interface {
	Config() error
	GetWorkspaces() []brev_api.Workspace
	GetConfiguredWorkspacePort(workspace brev_api.Workspace) string
}

func RunOn(sshConfigurer SSHConfigurer) error {
	err := sshConfigurer.Config()
	if err != nil {
		return err
	}

	workspaces := sshConfigurer.GetWorkspaces()

	err = sshall.RunSSHAll(workspaces, sshConfigurer.GetConfiguredWorkspacePort)
	if err != nil {
		return err
	}

	return nil
}
