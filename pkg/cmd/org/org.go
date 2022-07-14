package org

import (
	"fmt"
	"os"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/vpn"
	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/spf13/cobra"
)

type OrgCmdStore interface {
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetUsers(queryParams map[string]string) ([]entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	completions.CompletionStore
	vpn.ServiceMeshStore
	SetDefaultOrganization(org *entity.Organization) error
	GetServerSockFile() string
}

func NewCmdOrg(t *terminal.Terminal, orgcmdStore OrgCmdStore, noorgcmdStore OrgCmdStore) *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "org",
		Short:       "Organization commands",
		Long:        "Organization commands",
		Example: `
  brev org
  brev org ls
  brev org set <NAME>
		`,
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

	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org)")
	err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noorgcmdStore, t))
	if err != nil {
		t.Errprint(err, "cli err")
	}

	cmd.AddCommand(NewCmdOrgSet(t, orgcmdStore, noorgcmdStore))
	cmd.AddCommand(NewCmdOrgLs(t, orgcmdStore, noorgcmdStore))

	return cmd
}

func RunOrgs(t *terminal.Terminal, store OrgCmdStore) error {
	orgs, err := store.GetOrganizations(nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		t.Vprint(t.Yellow("You don't have any orgs. Create one! https://console.brev.dev"))
		return nil
	}

	defaultOrg, err := store.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprint(t.Yellow("Your organizations:"))
	displayOrgTable(t, orgs, defaultOrg)
	if len(orgs) > 1 {
		fmt.Print("\n")
		t.Vprintf(t.Green("Switch orgs:\n"))
		notDefaultOrg := getOtherOrg(orgs, *defaultOrg)
		// TODO suggest org with max workspaces
		t.Vprintf(t.Yellow("\tbrev org set <NAME> ex: brev org set %s\n", notDefaultOrg.Name))
	}

	return nil
}

func getBrevTableOptions() table.Options {
	options := table.OptionsDefault
	options.DrawBorder = false
	options.SeparateColumns = false
	options.SeparateRows = false
	options.SeparateHeader = false
	return options
}

func getOtherOrg(orgs []entity.Organization, org entity.Organization) *entity.Organization {
	for _, o := range orgs {
		if org.ID != o.ID {
			return &o
		}
	}
	return nil
}

func displayOrgTable(t *terminal.Terminal, orgs []entity.Organization, currentOrg *entity.Organization) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()
	header := table.Row{"NAME", "ID"}
	ta.AppendHeader(header)
	for _, o := range orgs {
		workspaceRow := []table.Row{{o.Name, o.ID}}
		if o.ID == currentOrg.ID {
			workspaceRow = []table.Row{{t.Green("* " + o.Name), t.Green(o.ID)}}
		}
		ta.AppendRows(workspaceRow)
	}
	ta.Render()
}
