package test

import (
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/vpn"

	"github.com/spf13/cobra"
)

var (
	startLong    = "[internal] test"
	startExample = "[internal] test"
)

type TestStore interface {
	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
}

type ServiceMeshStore interface {
	vpn.VPNStore
	GetCurrentWorkspaceID() string
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdTest(_ *terminal.Terminal, store ServiceMeshStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		Args:                  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ts := vpn.NewTailscale(store)
			if args[0] == "up" {
				nodeIdentifier := "me"
				workspaceID := store.GetCurrentWorkspaceID()
				if workspaceID != "" {
					workspace, err := store.GetWorkspace(workspaceID)
					if err != nil {
						return breverrors.WrapAndTrace(err)
					}
					localIdentifier := workspace.GetLocalIdentifier(nil)
					nodeIdentifier = string(localIdentifier)
				}

				err := ts.ApplyConfig(nodeIdentifier, config.GlobalConfig.GetServiceMeshCoordServerURL())
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
			}
			if args[0] == "start" {
				workspaceID := store.GetCurrentWorkspaceID()
				if workspaceID != "" {
					ts.WithUserspaceNetworking(true)
					ts.WithSockProxyPort(1055)
				}
				err := ts.Start()
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
			}
			return nil
		},
	}

	return cmd
}
