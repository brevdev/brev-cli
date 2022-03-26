package setupworkspace

import (
	"github.com/spf13/cobra"
)

// Internal command for setting up workspace
func NewCmdSetupWorkspace() *cobra.Command {
	cmd := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return cmd
}
