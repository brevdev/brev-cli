// Package set is for the set command
package set

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
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
	GetServerSockFile() string
	GetCurrentWorkspaceID() (string, error)
}

func NewCmdSet(t *terminal.Terminal, loginSetStore SetStore, noLoginSetStore SetStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:       map[string]string{"context": ""},
		Use:               "set",
		Short:             "Set active org (helps with completion)",
		Long:              "Set your organization to view, open, create dev environments etc",
		Example:           `brev set <org name>`,
		Args:              cmderrors.TransformToValidationError(cobra.MinimumNArgs(1)),
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
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func set(orgName string, setStore SetStore) error {
	workspaceID, err := setStore.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		return fmt.Errorf("can not set orgs in a workspace")
	}
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

	// Print workspaces within org

	return nil
}
