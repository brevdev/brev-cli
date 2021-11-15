package on

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/brev_errors"
	brevssh "github.com/brevdev/brev-cli/pkg/brev_ssh"
	"github.com/brevdev/brev-cli/pkg/cmd/sshall"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/spf13/afero"
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
		return brev_errors.WrapAndTrace(err)
	}

	fs := files.AppFs
	onStore := store.NewBasicStore(*config.NewConstants()).WithFileSystem(fs).WithAuthHTTPClient(store.NewAuthHTTPClient())

	workspaces, err := GetActiveWorkspaces(client, fs)

	sshConfigurer := brevssh.NewDefaultSSHConfigurer(workspaces, onStore, workspaceGroupClientMapper.GetPrivateKey())

	s.on = NewOn(workspaces, sshConfigurer, workspaceGroupClientMapper)
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
	GetConfiguredWorkspacePort(workspace brev_api.Workspace) (string, error)
}

type On struct {
	sshConfigurer              SSHConfigurer
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper
	workspaces                 []brev_api.WorkspaceWithMeta
}

func NewOn(workspaces []brev_api.WorkspaceWithMeta, sshConfigurer SSHConfigurer, workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper) *On {
	return &On{
		workspaces:                 workspaces,
		sshConfigurer:              sshConfigurer,
		workspaceGroupClientMapper: workspaceGroupClientMapper,
	}
}

func (o On) Run() error {
	err := o.sshConfigurer.Config()
	if err != nil {
		return err
	}

	sshall := sshall.NewSSHAll(o.workspaces, o.workspaceGroupClientMapper, o.sshConfigurer)
	err = sshall.Run()
	if err != nil {
		return err
	}

	return nil
}

func GetActiveWorkspaces(client *brev_api.Client, fs afero.Fs) ([]brev_api.WorkspaceWithMeta, error) {
	fmt.Println("Resolving workspaces...")

	org, err := brev_api.GetActiveOrgContext(fs)
	if err != nil {
		return nil, brev_errors.WrapAndTrace(err)
	}

	workspaces, err := client.GetMyWorkspaces(org.ID)
	if err != nil {
		return nil, brev_errors.WrapAndTrace(err)
	}

	var workspacesWithMeta []brev_api.WorkspaceWithMeta
	for _, w := range workspaces {
		wmeta, err := client.GetWorkspaceMetaData(w.ID)
		if err != nil {
			return nil, brev_errors.WrapAndTrace(err)
		}

		workspaceWithMeta := brev_api.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: w}
		workspacesWithMeta = append(workspacesWithMeta, workspaceWithMeta)
	}
	return workspacesWithMeta, nil
}
