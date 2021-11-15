package up

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	brevssh "github.com/brevdev/brev-cli/pkg/brevssh"
	"github.com/brevdev/brev-cli/pkg/cmd/sshall"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type onOptions struct {
	on *On
}

func NewCmdOn(_ *terminal.Terminal) *cobra.Command {
	// t *terminal.Terminal
	opts := onOptions{}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "up",
		DisableFlagsInUseLine: true,
		Short:                 "Set up a connection to all of your Brev workspaces",
		Long:                  "Set up a connection to all of your Brev workspaces",
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

func (s *onOptions) Complete(_ *cobra.Command, _ []string) error {
	// func (s *onOptions) Complete(cmd *cobra.Command, args []string) error {
	fmt.Println("Setting up client...")
	client, err := brevapi.NewCommandClient() // to resolve
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaceGroupClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(client) // to resolve
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fs := files.AppFs
	onStore := store.NewBasicStore(*config.NewConstants()).WithFileSystem(fs).WithAuthHTTPClient(store.NewAuthHTTPClient())

	workspaces, err := GetActiveWorkspaces(client, fs)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

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
	GetConfiguredWorkspacePort(workspace brevapi.Workspace) (string, error)
}

type On struct {
	sshConfigurer              SSHConfigurer
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper
	workspaces                 []brevapi.WorkspaceWithMeta
}

func NewOn(workspaces []brevapi.WorkspaceWithMeta, sshConfigurer SSHConfigurer, workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper) *On {
	return &On{
		workspaces:                 workspaces,
		sshConfigurer:              sshConfigurer,
		workspaceGroupClientMapper: workspaceGroupClientMapper,
	}
}

func (o On) Run() error {
	err := o.sshConfigurer.Config()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	sshall := sshall.NewSSHAll(o.workspaces, o.workspaceGroupClientMapper, o.sshConfigurer)
	err = sshall.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func GetActiveWorkspaces(client *brevapi.Client, fs afero.Fs) ([]brevapi.WorkspaceWithMeta, error) {
	fmt.Println("Resolving workspaces...")

	org, err := brevapi.GetActiveOrgContext(fs)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	workspaces, err := client.GetMyWorkspaces(org.ID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var workspacesWithMeta []brevapi.WorkspaceWithMeta
	for _, w := range workspaces {
		wmeta, err := client.GetWorkspaceMetaData(w.ID)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}

		workspaceWithMeta := brevapi.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: w}
		workspacesWithMeta = append(workspacesWithMeta, workspaceWithMeta)
	}
	return workspacesWithMeta, nil
}
