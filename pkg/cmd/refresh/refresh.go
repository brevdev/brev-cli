// Package refresh lists workspaces in the current org
package refresh

import (
	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func NewCmdRefresh(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"housekeeping": ""},
		Use:         "refresh",
		Short:       "Refresh the cache",
		Long:        "Refresh Brev's cache",
		Example:     `brev refresh`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := refresh(t)
			return err
			// return nil
		},
	}

	return cmd
}

func refresh(t *terminal.Terminal) error {
	err := brev_api.WriteCaches()
	if err != nil {
		return err
	}

	t.Vprintf(t.Green("Cache has been refreshed\n"))

	return nil
}
