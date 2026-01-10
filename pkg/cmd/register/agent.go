package register

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/terminal"
)

const (
	agentShort = "Run the Brev agent daemon"
	agentLong  = "Runs the Brev agent daemon in a continuous loop, reporting status and handling workloads."
)

func NewCmdBrevAgent(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: agentShort,
		Long:  agentLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrevAgent(t)
		},
	}
	return cmd
}

func runBrevAgent(t *terminal.Terminal) error {
	t.Vprint(t.Green("Starting Brev agent daemon...\n"))

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Print immediately on startup
	t.Vprint(fmt.Sprintf("[%s] Brev agent running\n", time.Now().Format(time.RFC3339)))

	// TODO this should call the logic in pkg/brevdaemon/agent.go
	for {
		select {
		case <-ticker.C:
			t.Vprint(fmt.Sprintf("[%s] Brev agent heartbeat\n", time.Now().Format(time.RFC3339)))
		}
	}
}
