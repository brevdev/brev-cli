package up

import (
	"fmt"

	brevssh "github.com/brevdev/brev-cli/pkg/brevssh"
	"github.com/brevdev/brev-cli/pkg/cmd/sshall"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type upOptions struct {
	on      *Up
	upStore UpStore
}

func NewCmdUp(upStore UpStore, t *terminal.Terminal) *cobra.Command {
	opts := upOptions{upStore: upStore}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "up",
		DisableFlagsInUseLine: true,
		Short:                 "Set up ssh connections to all of your Brev workspaces",
		Long:                  "Set up ssh connections to all of your Brev workspaces",
		Example:               "brev up",
		Args:                  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(t, cmd, args))
			cmdutil.CheckErr(opts.Validate(t))
			cmdutil.CheckErr(opts.RunOn(t))
		},
	}
	return cmd
}

func (s *upOptions) Complete(_ *terminal.Terminal, _ *cobra.Command, _ []string) error {
	// spinner := t.NewSpinner()
	// spinner.Suffix = "  Setting up client"
	// spinner.Start()

	workspaceGroupClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(s.upStore) // to resolve
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// spinner.Suffix = "  Resolving workspaces"
	workspaces, err := GetActiveWorkspaces(s.upStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	sshConfigurer := brevssh.NewDefaultSSHConfigurer(workspaces, s.upStore, workspaceGroupClientMapper.GetPrivateKey())

	s.on = NewUp(workspaces, sshConfigurer, workspaceGroupClientMapper)
	// spinner.Stop()
	return nil
}

type UpStore interface {
	brevssh.SSHStore
	k8s.K8sStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	GetCurrentUser() (*entity.User, error)
}

func (s upOptions) Validate(_ *terminal.Terminal) error {
	return nil
}

func (s upOptions) RunOn(_ *terminal.Terminal) error {
	fmt.Println("Running up...")
	return s.on.Run()
}

type SSHConfigurer interface {
	Config() error
	GetConfiguredWorkspacePort(workspace entity.Workspace) (string, error)
}

type Up struct {
	sshConfigurer              SSHConfigurer
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper
	workspaces                 []entity.WorkspaceWithMeta
}

func NewUp(workspaces []entity.WorkspaceWithMeta, sshConfigurer SSHConfigurer, workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper) *Up {
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

func GetActiveWorkspaces(upStore UpStore) ([]entity.WorkspaceWithMeta, error) {
	// fmt.Println("Resolving workspaces...")

	org, err := upStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, fmt.Errorf("no orgs exist")
	}

	user, err := upStore.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	workspaces, err := upStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{
		UserID: user.ID,
	})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var workspacesWithMeta []entity.WorkspaceWithMeta
	for _, w := range workspaces {
		wmeta, err := upStore.GetWorkspaceMetaData(w.ID)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}

		workspaceWithMeta := entity.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: w}
		workspacesWithMeta = append(workspacesWithMeta, workspaceWithMeta)
	}
	return workspacesWithMeta, nil
}
