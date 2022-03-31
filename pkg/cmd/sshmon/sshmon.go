package sshmon

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/analytics"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/spf13/cobra"
)

type SSHMonStore interface {
	analytics.SSHAnalyticsStore
}

func NewCmdSSHMon(store SSHMonStore, segmentAPIWriteKey string) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"hidden": ""},
		Use:         "sshmon",
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("running sshmon...")
			sshMonitor := analytics.NewSSHMonitor()
			segment := analytics.NewSegmentClient(segmentAPIWriteKey)
			defer segment.Client.Close() //nolint:errcheck // defer
			sshMontasks := []tasks.Task{&analytics.SSHAnalyticsTask{
				SSHMonitor: sshMonitor,
				Analytics:  segment,
				Store:      store,
			}}
			err := tasks.RunTasks(sshMontasks)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}
