// Package analytics provides the `brev analytics` command for managing usage analytics preferences.
package analytics

import (
	"github.com/brevdev/brev-cli/pkg/analytics"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

// AnalyticsStore is provided for parity with other commands; no store calls are required today.
type AnalyticsStore interface{}

func NewCmdAnalytics(t *terminal.Terminal, _ AnalyticsStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "analytics",
		DisableFlagsInUseLine: true,
		Short:                 "Manage usage analytics",
		Long:                  "Show or change whether the Brev CLI sends usage analytics.",
		Example:               "brev analytics\nbrev analytics on\nbrev analytics off",
		RunE: func(cmd *cobra.Command, args []string) error {
			printStatus(t)
			return nil
		},
	}

	cmd.AddCommand(newCmdOn(t))
	cmd.AddCommand(newCmdOff(t))

	return cmd
}

func newCmdOn(t *terminal.Terminal) *cobra.Command {
	return &cobra.Command{
		Use:                   "on",
		DisableFlagsInUseLine: true,
		Short:                 "Enable usage analytics",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := analytics.SetAnalyticsPreference(true); err != nil {
				return breverrors.WrapAndTrace(err)
			}
			t.Vprintf("%s\n", t.Green("Analytics enabled."))
			return nil
		},
	}
}

func newCmdOff(t *terminal.Terminal) *cobra.Command {
	return &cobra.Command{
		Use:                   "off",
		DisableFlagsInUseLine: true,
		Short:                 "Disable usage analytics",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := analytics.SetAnalyticsPreference(false); err != nil {
				return breverrors.WrapAndTrace(err)
			}
			t.Vprintf("%s\n", t.Green("Analytics disabled."))
			return nil
		},
	}
}

func printStatus(t *terminal.Terminal) {
	if disabled, varName := analytics.IsDisabledByEnv(); disabled {
		t.Vprintf("Analytics: %s (%s=1)\n", t.Yellow("disabled"), varName)
		return
	}

	if analytics.IsAnalyticsEnabled() {
		t.Vprintf("Analytics: %s\n", t.Green("enabled"))
		t.Vprintf("Run %s to opt out.\n", t.Yellow("brev analytics off"))
	} else {
		t.Vprintf("Analytics: %s\n", t.Yellow("disabled"))
		t.Vprintf("Run %s to opt in.\n", t.Yellow("brev analytics on"))
	}
}
