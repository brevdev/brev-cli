// Package ls lists workspaces in the current org
package org

import (
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type OrgStore interface {
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetUsers(queryParams map[string]string) ([]entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	SetDefaultOrganization(org *entity.Organization) error
}

func NewCmdOrg(t *terminal.Terminal, loginOrgStore OrgStore, noLoginOrgStore OrgStore) *cobra.Command {

	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "org",
		Short:       "Set, create, or list your orgs",
		Long:        "Set, create, or list your orgs",
		Example: `
  brev org ls
  brev org set <org_name_or_id>
  brev org new --name <org_name_or_id>
		`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{}, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveDefault
		},
	}

	// cmd.AddCommand(newCmdSet(t))
	// cmd.AddCommand(newCmdLs(t))
	cmd.AddCommand(newCmdNew(t))

	// cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org)")
	// err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noLoginOrgStore, t))
	// if err != nil {
	// 	t.Errprint(err, "cli err")
	// }
	// cmd.Flags().BoolVar(&showAll, "all", false, "show all workspaces in org")

	return cmd
}

func newCmdNew(t *terminal.Terminal) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:     "new",
		Short:   "Create a new org",
		Long:    "Create a new org",
		Example: `  brev org new --name NewEp`,
		RunE: func(cmd *cobra.Command, args []string) error {
			t.Vprintf("\nYou wanna create an org")
			// return addEndpoint(name, t)
			return nil
		},
		Args: cobra.MinimumNArgs(0),
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "name of the org")

	return cmd
}

func RunOrg(t *terminal.Terminal, os OrgStore, args []string, orgflag string, showAll bool) error {

	switch args[0] {
	case "ls":
		err := ListOrgs(os, t)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	case "set":
		err := SetOrg(os, t, args[2])
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	case "new":
		err := CreateOrg(os, t)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	return nil
}

func CreateOrg(os OrgStore, t *terminal.Terminal) error {
	orgs, err := os.GetOrganizations(nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		t.Vprint(t.Yellow("You don't have any orgs. Create one!"))
		return nil
	}

	defaultOrg, err := os.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprint(t.Yellow("Your organizations:"))
	displayOrgs(t, orgs, defaultOrg)

	return nil
}

func SetOrg(os OrgStore, t *terminal.Terminal, orgName string) error {
	orgs, err := os.GetOrganizations(nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		t.Vprint(t.Yellow("You don't have any orgs. Create one!"))
		return nil
	} else if len(orgs) > 1 {
		return fmt.Errorf("more than one org exist with name %s", orgName)
	}

	org := orgs[0]

	err = os.SetDefaultOrganization(&org)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Print workspaces within org
	return nil
}

func ListOrgs(os OrgStore, t *terminal.Terminal) error {
	orgs, err := os.GetOrganizations(nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		t.Vprint(t.Yellow("You don't have any orgs. Create one!"))
		return nil
	}

	defaultOrg, err := os.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprint(t.Yellow("Your organizations:"))
	displayOrgs(t, orgs, defaultOrg)

	return nil
}

func displayOrgs(t *terminal.Terminal, organizations []entity.Organization, defaultOrg *entity.Organization) {
	idLen := 9
	if len(organizations) > 0 {
		t.Vprint("  ID" + strings.Repeat(" ", idLen+1-len("ID")) + "NAME")
		for _, v := range organizations {
			if defaultOrg.ID == v.ID {
				t.Vprint(t.Green("* " + truncateString(v.ID, idLen) + strings.Repeat(" ", idLen-len(truncateString(v.ID, idLen))) + " " + v.Name))
			} else {
				t.Vprint("  " + truncateString(v.ID, idLen) + strings.Repeat(" ", idLen-len(truncateString(v.ID, idLen))) + " " + v.Name)
			}
		}
	}
}

func truncateString(s string, delimterCount int) string {
	if len(s) <= delimterCount {
		return s
	} else {
		return s[:delimterCount]
	}
}
