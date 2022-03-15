package test

import (
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/server"
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
	CopyBin(targetBin string) error
	GetSetupScriptContentsByURL(url string) (string, error)
	server.RPCServerTaskStore
}

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
	GetCurrentWorkspaceID() string
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdTest(_ *terminal.Terminal, store TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		Args:                  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// gistURL := "https://gist.githubusercontent.com/naderkhalil/4a45d4d293dc3a9eb330adcd5440e148/raw/3ab4889803080c3be94a7d141c7f53e286e81592/setup.sh"

			// resp, err := store.GetSetupScriptContentsByURL(gistURL)
			// if err != nil {
			// 	return nil
			// }
			// fmt.Println(resp)
			if args[0] == "s" {
				s := server.RPCServerTask{store}
				return s.Run()

			}
			if args[0] == "c" {
				return vpn.ConfigureVPN(store)
			}
			return nil

			// return nil
			// cfg := autostartconf.LinuxSystemdConfigurer{
			// 	AutoStartStore: store,
			// 	ValueConfigFile: `
			// [Install]
			// WantedBy=multi-user.target

			// [Unit]
			// Description=Brev SSH Proxy Daemon
			// After=systemd-user-sessions.service

			// [Service]
			// Type=simple
			// ExecStart=brev run-tasks
			// Restart=always
			// `, DestConfigFile: "/etc/systemd/system/brev.service",
			// }
			// err := cfg.Install()
			// if err != nil {
			// 	return breverrors.WrapAndTrace(err)
			// }
			// return nil
		},
	}

	return cmd
}
