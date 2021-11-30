package completions

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

type CompletionStore interface {
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]brevapi.Workspace, error)
	GetActiveOrganizationOrDefault() (*brevapi.Organization, error)
	GetCurrentUser() (*brevapi.User, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]brevapi.Organization, error)
}

type CompletionHandler func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

func GetAllWorkspaceNameCompletionHandler(completionStore CompletionStore, t *terminal.Terminal) CompletionHandler {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		user, err := completionStore.GetCurrentUser()
		if err != nil {
			t.Errprint(err, "")
			return nil, cobra.ShellCompDirectiveError
		}

		org, err := completionStore.GetActiveOrganizationOrDefault()
		if err != nil {
			t.Errprint(err, "")
			return nil, cobra.ShellCompDirectiveError
		}
		if org == nil {
			return []string{}, cobra.ShellCompDirectiveDefault
		}

		workspaces, err := completionStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{UserID: user.ID})
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

func GetOrgsNameCompletionHandler(completionStore CompletionStore, t *terminal.Terminal) CompletionHandler {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
