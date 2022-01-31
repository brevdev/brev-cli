// Package update is for the set command
package update

import (
	"fmt"
	"os/exec"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type UpdateStore interface {
	completions.CompletionStore
	SetDefaultOrganization(org *entity.Organization) error
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
}

func NewCmdUpdate(t *terminal.Terminal, loginUpdateStore UpdateStore, noLoginUpdateStore UpdateStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:       map[string]string{"housekeeping": ""},
		Use:               "update",
		Short:             "update your CLI",
		Long:              "update your CLI",
		Example:           `brev update`,
		ValidArgsFunction: completions.GetOrgsNameCompletionHandler(noLoginUpdateStore, t),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// sudo sh -c "`curl -sf -L https://raw.githubusercontent.com/brevdev/brev-cli/main/bin/install-latest.sh`" && export PATH=/opt/brev/bin:$PATH && brev run-tasks -d
			//
			command := exec.Command("sudo", "sh", "-c", "\"`curl -sf -L https://raw.githubusercontent.com/brevdev/brev-cli/main/bin/install-latest.sh`\"") // #nosec G204
			err := command.Run()
			fmt.Println(command.Stdout)
			fmt.Println(command.Stderr)
			fmt.Println(err.Error())
			return breverrors.WrapAndTrace(err)
			// err := set(args[0], loginUpdateStore)
			// return breverrors.WrapAndTrace(err)
		},
	}

	return cmd
}
