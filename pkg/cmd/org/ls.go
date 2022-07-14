package org

import (
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func NewCmdOrgLs(t *terminal.Terminal, orgcmdStore OrgCmdStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "ls",
		Short:       "List your organizations",
		Long: `List your organizations, your current org will be prefixed
with * and highlighted with green`,
		Example: `brev org ls`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
		Args: cmderrors.TransformToValidationError(cobra.NoArgs),
		// ValidArgs: []string{"new", "ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunOrgs(t, orgcmdStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}
