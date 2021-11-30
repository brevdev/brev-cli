// Package set is for the set command
package set

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func getOrgs() []brevapi.Organization {
	client, err := brevapi.NewDeprecatedCommandClient()
	if err != nil {
		panic(err)
	}
	orgs, err := client.GetOrgs()
	if err != nil {
		panic(err)
	}

	return orgs
}

type SetStore interface {
	completions.CompletionStore
	SetDefaultOrganization(org *brevapi.Organization) error
	GetOrganizations(options *store.GetOrganizationsOptions) ([]brevapi.Organization, error)
}

func NewCmdSet(t *terminal.Terminal, loginSetStore SetStore, noLoginSetStore SetStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:       map[string]string{"context": ""},
		Use:               "set",
		Short:             "Set active org (helps with completion)",
		Long:              "Set your organization to view, open, create workspaces etc",
		Example:           `brev set [org name]`,
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
			err := set(args[0], loginSetStore)
			return breverrors.WrapAndTrace(err)
		},
	}

	return cmd
}

func set(orgName string, setStore SetStore) error {
	orgs, err := setStore.GetOrganizations(&store.GetOrganizationsOptions{Name: orgName})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		return fmt.Errorf("no orgs exist with name %s", orgName)
	} else if len(orgs) > 1 {
		return fmt.Errorf("more than one org exist with name %s", orgName)
	}

	org := orgs[0]

	err = setStore.SetDefaultOrganization(&org)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
