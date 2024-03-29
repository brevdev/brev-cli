package org

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func NewCmdOrgSet(t *terminal.Terminal, orgcmdStore OrgCmdStore, noorgcmdStore OrgCmdStore) *cobra.Command {
	var showAll bool
	var org string

	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "set",
		Short:       "Set active org",
		Long:        "Set your organization to view, open, create workspaces etc",
		Example: `
  brev org set <NAME>
		`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		Args: cmderrors.TransformToValidationError(cobra.MinimumNArgs(1)),
		// ValidArgs: []string{"new", "ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := set(args[0], orgcmdStore, t)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org)")
	err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noorgcmdStore, t))
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err))
		fmt.Print(breverrors.WrapAndTrace(err))
	}

	cmd.Flags().BoolVar(&showAll, "all", false, "show all workspaces in org")

	return cmd
}

func set(orgName string, setStore OrgCmdStore, t *terminal.Terminal) error {
	workspaceID, err := setStore.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println(workspaceID)

	if workspaceID != "" {
		return breverrors.NewValidationError("can not set orgs in a workspace")
	}
	orgs, err := setStore.GetOrganizations(&store.GetOrganizationsOptions{Name: orgName})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		return breverrors.NewValidationError(fmt.Sprintf("no orgs exist with name %s", orgName))
	} else if len(orgs) > 1 {
		return breverrors.NewValidationError(fmt.Sprintf("more than one org exist with name %s", orgName))
	}

	org := orgs[0]

	err = setStore.SetDefaultOrganization(&org)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprintf("Org %s is now active 🤙\n", t.Green(org.Name))

	// Print workspaces within org

	return nil
}
