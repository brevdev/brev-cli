// Package refresh lists workspaces in the current org
package refresh

import (
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/ssh"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type RefreshStore interface {
	ssh.ConfigUpdaterStore
	ssh.SSHConfigurerV2Store
}

func NewCmdRefresh(t *terminal.Terminal, store RefreshStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"housekeeping": ""},
		Use:         "refresh",
		Short:       "Refresh the cache",
		Long:        "Refresh Brev's cache",
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
			// return nil
		},
	}

	return cmd
}

func refresh(t *terminal.Terminal, store RefreshStore) error {
	bar := t.NewProgressBar("Fetching orgs and workspaces", func() {})
	bar.AdvanceTo(50)

	cu := ssh.ConfigUpdater{
		Store: store,
		Configs: []ssh.Config{
			ssh.NewSSHConfigurerV2(
				store,
			),
		},
	}

	err := cu.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	bar.AdvanceTo(100)
	t.Vprintf(t.Green("\nCache has been refreshed\n"))

	return nil
}
