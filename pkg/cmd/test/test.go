package test

import (
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

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
	CopyBin(targetBin string) error
}

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
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
		// Args:                  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := autostartconf.LinuxSystemdConfigurer{
				AutoStartStore: store,
				ValueConfigFile: `
[Install]
WantedBy=multi-user.target

[Unit]
Description=Brev SSH Proxy Daemon
After=systend-user-sessions.service

[Service]
Type=simple
ExecStart=brev run-tasks
Restart=always
`, DestConfigFile: "/etc/systemd/system/brev.service",
			}
			err := cfg.Install()
			if err != nil {
				return err
			}
			return nil
		},
	}

	return cmd
}
