package printconfig

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func NewCmdPrintConfig(t *terminal.Terminal, store refresh.RefreshStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"configuration": ""},
		Use:         "print-config",
		Short:       "Print the SSH config Brev expects you to add manually",
		Long:        "Print the Include directive and generated Brev SSH config without modifying any files.",
		Example:     "brev print-config",
		Args:        cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			preview, err := refresh.BuildSSHConfigPreview(store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			t.Vprintf("# Add this to your SSH config (for example via Home Manager)\n")
			t.Vprintf("%s\n", preview.IncludeDirective)
			t.Vprintf("# Brev-managed SSH config at %s\n", preview.BrevConfigPath)
			fmt.Print(preview.BrevConfig)
			return nil
		},
	}

	return cmd
}
