// Package analytics exposes the `brev analytics` command for managing the
// user's opt-in preference for anonymous usage analytics.
package analytics

import (
	"fmt"

	analyticspkg "github.com/brevdev/brev-cli/pkg/analytics"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

// NewCmdAnalytics returns the `brev analytics` command with enable/disable/status subcommands.
func NewCmdAnalytics(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "analytics",
		DisableFlagsInUseLine: true,
		Short:                 "Manage anonymous usage analytics",
		Long: `Enable, disable, or check the status of anonymous usage analytics.

Analytics are opt-in. When enabled, Brev reports command usage and error
rates to help the team prioritize fixes and improvements. No command
arguments, file contents, or credentials are ever captured.`,
		Example: "brev analytics enable\nbrev analytics disable\nbrev analytics status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(t)
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:     "enable",
		Short:   "Opt in to anonymous usage analytics",
		Example: "brev analytics enable",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSet(t, true)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:     "disable",
		Short:   "Opt out of anonymous usage analytics",
		Example: "brev analytics disable",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSet(t, false)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:     "status",
		Short:   "Show whether anonymous usage analytics are enabled",
		Example: "brev analytics status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(t)
		},
	})

	return cmd
}

func runSet(t *terminal.Terminal, enabled bool) error {
	if err := analyticspkg.SetAnalyticsPreference(enabled); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	analyticspkg.CaptureAnalyticsOptIn(enabled)
	if enabled {
		t.Vprintf("%s Analytics enabled. Thanks for helping improve Brev.\n", t.Green("✓"))
	} else {
		t.Vprintf("%s Analytics disabled.\n", t.Green("✓"))
	}
	return nil
}

func runStatus(t *terminal.Terminal) error {
	enabled, asked := analyticspkg.IsAnalyticsEnabled()
	switch {
	case !asked:
		fmt.Println("Analytics: not configured (off by default).")
		t.Vprintf("Run %s to opt in.\n", t.Yellow("brev analytics enable"))
	case enabled:
		fmt.Println("Analytics: enabled.")
		t.Vprintf("Run %s to opt out.\n", t.Yellow("brev analytics disable"))
	default:
		fmt.Println("Analytics: disabled.")
		t.Vprintf("Run %s to opt in.\n", t.Yellow("brev analytics enable"))
	}
	return nil
}
