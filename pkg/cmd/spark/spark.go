package spark

import (
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

const (
	sparkShort = "Manage NVIDIA Spark connectivity"
	sparkLong  = "Commands for connecting Brev to NVIDIA Spark environments."
)

// NewCmdSpark is the root Spark command; subcommands (e.g., ssh, connect)
// are added beneath it.
func NewCmdSpark(t *terminal.Terminal, loginCmdStore *store.AuthHTTPStore, noLoginCmdStore *store.AuthHTTPStore) *cobra.Command {
	// Keep stores for future subcommands (e.g., connect) even if unused now.
	_ = loginCmdStore
	_ = noLoginCmdStore

	cmd := &cobra.Command{
		Use:   "spark",
		Short: sparkShort,
		Long:  sparkLong,
	}

	cmd.AddCommand(NewCmdSparkSSH(t))
	cmd.AddCommand(NewCmdSparkEnroll(t, loginCmdStore))
	cmd.AddCommand(NewCmdSparkAgent(t))

	return cmd
}
