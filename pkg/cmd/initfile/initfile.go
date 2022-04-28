package initfile

import (
	"github.com/brevdev/brev-cli/pkg/mergeshells" //nolint:typecheck // uses generic code
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

type InitFileStore interface {
	GetFileAsString(path string) (string, error)
}

func NewCmdInitFile(t *terminal.Terminal, store InitFileStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "init",
		DisableFlagsInUseLine: true,
		Short:                 "initialize a .brev/setup.sh file if it does not exist",
		Long:                  "initialize a .brev/setup.sh file if it does not exist",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				// then assume it is .
				mergeshells.ImportPath(args[0])
			} else {
				mergeshells.ImportPath(".")
			}
		},
	}

	return cmd
}
