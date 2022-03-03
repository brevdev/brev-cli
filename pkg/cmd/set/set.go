// Package set is for the set command
package set

import (
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type SetStore interface {
	completions.CompletionStore
	SetDefaultOrganization(org *entity.Organization) error
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
}

func NewCmdSet(t *terminal.Terminal, loginSetStore SetStore, noLoginSetStore SetStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:       map[string]string{"hidden": ""},
		Use:               "set",
		Short:             "Set active org (helps with completion)",
		Long:              "Set your organization to view, open, create workspaces etc",
		Example:           `brev org set [org name]`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completions.GetOrgsNameCompletionHandler(noLoginSetStore, t),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			t.Vprintf(t.Yellow("This command changed, please run:\n\tbrev org set %s"), args[0])
			return nil
		},
	}

	return cmd
}
