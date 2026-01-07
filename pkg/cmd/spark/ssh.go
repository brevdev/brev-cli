package spark

import (
	"strings"

	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	sparklib "github.com/brevdev/brev-cli/pkg/spark"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

const (
	sparkSSHShort = "SSH into a Spark host discovered by NVIDIA Sync"
	sparkSSHLong  = "Locate the NVIDIA Sync ssh_config, discover Spark hosts, and open an SSH session."
)

func NewCmdSparkSSH(t *terminal.Terminal) *cobra.Command {
	var printCmd bool

	cmd := &cobra.Command{
		Use:                   "ssh [host-alias]",
		Short:                 sparkSSHShort,
		Long:                  sparkSSHLong,
		Args:                  cobra.MaximumNArgs(1),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := ""
			if len(args) > 0 {
				alias = args[0]
			}

			err := runSparkSSH(t, alias, printCmd)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&printCmd, "print-cmd", false, "Print the resolved ssh command before running")

	return cmd
}

func runSparkSSH(t *terminal.Terminal, alias string, printCmd bool) error {
	locator := sparklib.NewSyncSSHConfigLocator()
	resolver := sparklib.NewDefaultSyncConfigResolver(files.AppFs, locator)
	hosts, err := resolver.ResolveHosts()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	selected, err := sparklib.SelectHost(hosts, alias, sparklib.TerminalPrompter{})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if printCmd {
		args := sparklib.BuildSSHArgs(selected)
		t.Vprint(strings.Join(args, " "))
	}

	runner := sparklib.NewDefaultSSHRunner(files.AppFs)
	return breverrors.WrapAndTrace(runner.Run(selected))
}
