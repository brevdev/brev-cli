// Package refresh lists workspaces in the current org
package refresh

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/ssh"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type RefreshStore interface {
	ssh.ConfigUpdaterStore
	ssh.SSHConfigurerV2Store
	GetCurrentUser() (*entity.User, error)
}

func NewCmdRefresh(t *terminal.Terminal, store RefreshStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"housekeeping": ""},
		Use:         "refresh",
		Short:       "Force a refresh to the ssh config",
		Long:        "Force a refresh to the ssh config",
		Example:     `brev refresh`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := refresh(t, store)
			return breverrors.WrapAndTrace(err)
		},
	}

	return cmd
}

func refresh(t *terminal.Terminal, store RefreshStore) error {
	fmt.Println("refreshing brev...")
	cu, err := getConfigUpdater(store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = cu.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprintf(t.Green("brev has been refreshed\n"))

	return nil
}

func getConfigUpdater(store RefreshStore) (*ssh.ConfigUpdater, error) {
	configs, err := ssh.GetSSHConfigs(store)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	cu := ssh.ConfigUpdater{
		Store:   store,
		Configs: configs,
	}

	return &cu, nil
}
