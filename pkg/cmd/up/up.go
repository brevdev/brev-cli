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
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type upOptions struct {
	on *Up
}

func NewCmdUp(_ *terminal.Terminal) *cobra.Command {
	// t *terminal.Terminal
	opts := upOptions{}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "up",
		DisableFlagsInUseLine: true,
		Short:                 "Set up ssh connections to all of your Brev workspaces",
		Long:                  "Set up ssh connections to all of your Brev workspaces",
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

func (s *upOptions) Complete(_ *cobra.Command, _ []string) error {
	// func (s *onOptions) Complete(cmd *cobra.Command, args []string) error {
	fmt.Println("Setting up client...")
	client, err := brevapi.NewCommandClient() // to resolve
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	oauthToken, err := brevapi.GetToken()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	conf := config.NewConstants()
	fs := files.AppFs
	upStore := store.
		NewBasicStore().
		WithFileSystem(fs).
		WithAuthHTTPClient(store.NewAuthHTTPClient(client, oauthToken.AccessToken, conf.GetBrevAPIURl()))

	workspaceGroupClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(upStore) // to resolve
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspaces, err := GetActiveWorkspaces(upStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	sshConfigurer := brevssh.NewDefaultSSHConfigurer(workspaces, upStore, workspaceGroupClientMapper.GetPrivateKey())

	s.on = NewUp(workspaces, sshConfigurer, workspaceGroupClientMapper)
	return nil
}

type UpStore interface {
	GetActiveOrganizationOrDefault() (*brevapi.Organization, error)
	GetWorkspaces(organizationID string, options store.GetWorkspacesOptions) ([]brevapi.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*brevapi.WorkspaceMetaData, error)
	GetCurrentUser() (*brevapi.User, error)
}

func (s upOptions) Validate() error {
	return nil
}

func (s upOptions) RunOn() error {
	fmt.Println("Running up...")
	return s.on.Run()
}

type SSHConfigurer interface {
	Config() error
	GetConfiguredWorkspacePort(workspace brevapi.Workspace) (string, error)
}

type Up struct {
	sshConfigurer              SSHConfigurer
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper
	workspaces                 []brevapi.WorkspaceWithMeta
}

func NewUp(workspaces []brevapi.WorkspaceWithMeta, sshConfigurer SSHConfigurer, workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper) *Up {
	return &Up{
		workspaces:                 workspaces,
		sshConfigurer:              sshConfigurer,
		workspaceGroupClientMapper: workspaceGroupClientMapper,
	}
}

func (o Up) Run() error {
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

func GetActiveWorkspaces(upStore UpStore) ([]brevapi.WorkspaceWithMeta, error) {
	fmt.Println("Resolving workspaces...")

	org, err := upStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	user, err := upStore.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	workspaces, err := upStore.GetWorkspaces(org.ID, store.GetWorkspacesOptions{
		UserID: user.ID,
	})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var workspacesWithMeta []brevapi.WorkspaceWithMeta
	for _, w := range workspaces {
		wmeta, err := upStore.GetWorkspaceMetaData(w.ID)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}

		workspaceWithMeta := brevapi.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: w}
		workspacesWithMeta = append(workspacesWithMeta, workspaceWithMeta)
	}
	return workspacesWithMeta, nil
}
