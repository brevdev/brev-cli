package org

import (
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

func NewCmdOrgCreate(t *terminal.Terminal, orgcmdStore OrgCmdStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "create",
		Short:       "Create a new organization",
		Long:        "Create a new organization with the specified name",
		Example: `
  brev org create my-new-org
		`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
		Args: cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := createOrg(args[0], orgcmdStore, t)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func createOrg(orgName string, createStore OrgCmdStore, t *terminal.Terminal) error {
	if len(orgName) == 0 {
		return breverrors.NewValidationError("organization name cannot be empty")
	}

	startTime := time.Now()

	t.Vprintf("Creating organization %s...\n", t.Green(orgName))

	org, err := createStore.CreateOrganization(store.CreateOrganizationRequest{
		Name: orgName,
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	duration := time.Since(startTime)
	fmt.Print("\n")
	t.Vprint(t.Green(fmt.Sprintf("✓ Successfully created organization %s (ID: %s) in %v\n", org.Name, org.ID, duration.Round(time.Millisecond))))
	fmt.Print("\n")
	t.Vprint(t.Yellow("Switch to this organization:\n"))
	t.Vprint(t.Yellow(fmt.Sprintf("\tbrev org set %s\n", org.Name)))

	return nil
}
