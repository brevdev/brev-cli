package on

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	brevssh "github.com/brevdev/brev-cli/pkg/brev_ssh"
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
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "on",
		DisableFlagsInUseLine: true,
		Short:                 "on",
		Long:                  "on",
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
	fmt.Println("Setting up client...")
	client, err := brev_api.NewCommandClient() // to resolve
	if err != nil {
		return err
	}
	workspaceGroupClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(client) // to resolve
	if err != nil {
		return err
	}

	sshConfigurer := brevssh.NewDefaultSSHConfigurer(client, workspaceGroupClientMapper.GetPrivateKey(), brev_api.GetActiveOrgContext)

	s.on = NewOn(sshConfigurer, workspaceGroupClientMapper)
	return nil
}

func (s onOptions) Validate() error {
	return nil
}

func (s onOptions) RunOn() error {
	fmt.Println("Running on...")
	return s.on.Run()
}

type SSHConfigurer interface {
	Config() error
	GetWorkspaces() ([]brev_api.WorkspaceWithMeta, error)
	GetConfiguredWorkspacePort(workspace brev_api.Workspace) (string, error)
}

type On struct {
	sshConfigurer              SSHConfigurer
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper
}

func NewOn(sshConfigurer SSHConfigurer, workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper) *On {
	return &On{
		sshConfigurer:              sshConfigurer,
		workspaceGroupClientMapper: workspaceGroupClientMapper,
	}
}

func (o On) Run() error {
	err := o.sshConfigurer.Config()
	if err != nil {
		return err
	}

	sshall := sshall.NewSSHAll(o.workspaceGroupClientMapper, o.sshConfigurer)
	err = sshall.Run()
	if err != nil {
		return err
	}

	return nil
}
