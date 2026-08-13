package completions

import (
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

type CompletionStore interface {
	auth.APIKeyAuthStore
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
}

type CompletionHandler func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

func GetAllWorkspaceNameCompletionHandler(completionStore CompletionStore, t *terminal.Terminal) CompletionHandler {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		org, err := getOrganizationForCompletion(cmd, completionStore)
		if err != nil {
			t.Errprint(err, "")
			return nil, cobra.ShellCompDirectiveError
		}
		if org == nil {
			return []string{}, cobra.ShellCompDirectiveDefault
		}

		var options *store.GetWorkspacesOptions
		if !auth.IsAPIKeyAuthStore(completionStore) {
			user, err := completionStore.GetCurrentUser()
			if err != nil {
				t.Errprint(err, "")
				return nil, cobra.ShellCompDirectiveError
			}
			options = &store.GetWorkspacesOptions{UserID: user.ID}
		}

		workspaces, err := completionStore.GetWorkspaces(org.ID, options)
		if err != nil {
			t.Errprint(err, "")
			return nil, cobra.ShellCompDirectiveError
		}

		workspaceNames := []string{}
		for _, w := range workspaces {
			workspaceNames = append(workspaceNames, w.Name)
		}

		return workspaceNames, cobra.ShellCompDirectiveDefault
	}
}

func getOrganizationForCompletion(cmd *cobra.Command, completionStore CompletionStore) (*entity.Organization, error) {
	orgFlag := cmd.Flag("org")
	if orgFlag == nil || strings.TrimSpace(orgFlag.Value.String()) == "" {
		org, err := completionStore.GetActiveOrganizationOrDefault()
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		return org, nil
	}
	if auth.IsAPIKeyAuthStore(completionStore) {
		return nil, fmt.Errorf("%s", auth.APIKeyOrganizationOverrideNotSupportedMessage)
	}

	orgName := strings.TrimSpace(orgFlag.Value.String())
	orgs, err := completionStore.GetOrganizations(&store.GetOrganizationsOptions{Name: orgName})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		return nil, fmt.Errorf("no org found with name %s", orgName)
	}
	if len(orgs) > 1 {
		return nil, fmt.Errorf("more than one org found with name %s", orgName)
	}
	return &orgs[0], nil
}

func GetOrgsNameCompletionHandler(completionStore CompletionStore, t *terminal.Terminal) CompletionHandler {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if auth.IsAPIKeyAuthStore(completionStore) {
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}

		orgs, err := completionStore.GetOrganizations(nil)
		if err != nil {
			t.Errprint(err, "")
			return nil, cobra.ShellCompDirectiveError
		}

		orgNames := []string{}
		for _, o := range orgs {
			orgNames = append(orgNames, o.Name)
		}

		return orgNames, cobra.ShellCompDirectiveDefault
	}
}
