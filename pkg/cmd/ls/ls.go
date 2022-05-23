// Package ls lists workspaces in the current org
package ls

import (
	"fmt"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/util"
	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/spf13/cobra"
)

type LsStore interface {
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetUsers(queryParams map[string]string) ([]entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
}

func NewCmdLs(t *terminal.Terminal, loginLsStore LsStore, noLoginLsStore LsStore) *cobra.Command {
	var showAll bool
	var org string

	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "ls",
		Short:       "List workspaces within active org",
		Long:        "List workspaces within your active org. List all workspaces if no active org is set.",
		Example: `
  brev ls
  brev ls orgs
  brev ls --org <orgid>
		`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		Args:      cobra.MinimumNArgs(0),
		ValidArgs: []string{"orgs", "workspaces"},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunLs(t, loginLsStore, args, org, showAll)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org)")
	err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noLoginLsStore, t))
	if err != nil {
		t.Errprint(err, "cli err")
	}

	cmd.Flags().BoolVar(&showAll, "all", false, "show all workspaces in org")

	return cmd
}

func getOrgForRunLs(lsStore LsStore, orgflag string) (*entity.Organization, error) {
	var org *entity.Organization
	if orgflag != "" {
		var orgs []entity.Organization
		orgs, err := lsStore.GetOrganizations(&store.GetOrganizationsOptions{Name: orgflag})
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if len(orgs) == 0 {
			return nil, fmt.Errorf("no org found with name %s", orgflag)
		} else if len(orgs) > 1 {
			return nil, fmt.Errorf("more than one org found with name %s", orgflag)
		}

		org = &orgs[0]
	} else {
		var currOrg *entity.Organization
		currOrg, err := lsStore.GetActiveOrganizationOrDefault()
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if currOrg == nil {
			return nil, fmt.Errorf("no orgs exist")
		}
		org = currOrg
	}
	return org, nil
}

func RunLs(t *terminal.Terminal, lsStore LsStore, args []string, orgflag string, showAll bool) error {
	ls := NewLs(lsStore, t)
	user, err := lsStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	org, err := getOrgForRunLs(lsStore, orgflag)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(args) > 1 {
		return fmt.Errorf("too many args provided")
	}

	if len(args) == 1 { //nolint:gocritic // don't want to switch
		err = handleLsArg(ls, args[0], user, org, showAll)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else if len(args) == 0 {
		err = ls.RunWorkspaces(org, user, showAll)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		return fmt.Errorf("unhandle ls arguments")
	}

	return nil
}

func handleLsArg(ls *Ls, arg string, user *entity.User, org *entity.Organization, showAll bool) error {
	// todo refactor this to cmd.register
	//nolint:gocritic // idk how to write this as a switch
	if util.IsSingularOrPlural(arg, "org") || util.IsSingularOrPlural(arg, "organization") { // handle org, orgs, and organization(s)
		err := ls.RunOrgs()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		return nil
	} else if util.IsSingularOrPlural(arg, "workspace") {
		err := ls.RunWorkspaces(org, user, showAll)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else if util.IsSingularOrPlural(arg, "user") && featureflag.IsAdmin(user.GlobalUserType) {
		err := ls.RunUser(showAll)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		return nil
	} else if util.IsSingularOrPlural(arg, "host") && featureflag.IsAdmin(user.GlobalUserType) {
		err := ls.RunHosts(org)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		return nil
	}
	return nil
}

type Ls struct {
	lsStore  LsStore
	terminal *terminal.Terminal
}

func NewLs(lsStore LsStore, terminal *terminal.Terminal) *Ls {
	return &Ls{
		lsStore:  lsStore,
		terminal: terminal,
	}
}

func (ls Ls) RunOrgs() error {
	orgs, err := ls.lsStore.GetOrganizations(nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		ls.terminal.Vprint(ls.terminal.Yellow("You don't have any orgs. Create one!"))
		return nil
	}

	defaultOrg, err := ls.lsStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	ls.terminal.Vprint(ls.terminal.Yellow("Your organizations:"))
	displayOrgs(ls.terminal, orgs, defaultOrg)

	return nil
}

func (ls Ls) RunUser(_ bool) error {
	params := make(map[string]string)
	params["verificationStatus"] = "UnVerified"
	users, err := ls.lsStore.GetUsers(params)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	for _, user := range users {
		fmt.Printf("%s	%s	%s\n", user.ID, user.Name, user.Email)
	}

	return nil
}

func (ls Ls) ShowAllWorkspaces(org *entity.Organization, user *entity.User) error {
	err := ls.ShowUserWorkspaces(org, user)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	wss, err := ls.lsStore.GetWorkspaces(org.ID, nil) // TODO double network request
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	projects := entity.NewVirtualProjects(wss)

	var unjoinedProjects []entity.VirtualProject
	for _, p := range projects {
		wks := p.GetUserWorkspaces(user.ID)
		if len(wks) == 0 {
			unjoinedProjects = append(unjoinedProjects, p)
		}
	}

	displayUnjoinedProjects(ls.terminal, org.Name, unjoinedProjects)
	return nil
}

func (ls Ls) ShowUserWorkspaces(org *entity.Organization, user *entity.User) error {
	var workspaces []entity.Workspace
	workspaces, err := ls.lsStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{UserID: user.ID})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if len(workspaces) == 0 {
		ls.terminal.Vprint(ls.terminal.Yellow("No workspaces in org %s\n", org.Name))
		return nil
	}

	ls.terminal.Vprintf("You have %d workspaces in Org "+ls.terminal.Yellow(org.Name)+"\n", len(workspaces))
	displayWorkspaces(ls.terminal, workspaces)

	fmt.Print("\n")

	ls.terminal.Vprintf(ls.terminal.Green("Connect to running workspace:\n"))
	ls.terminal.Vprintf(ls.terminal.Yellow("\tbrev shell <name> ex: brev shell %s\n", workspaces[0].Name))
	ls.terminal.Vprintf(ls.terminal.Yellow("\tbrev open <name> ex: brev open %s\n", workspaces[0].Name))

	ls.terminal.Vprintf(ls.terminal.Green("Or ssh:\n"))
	for _, v := range workspaces {
		if v.Status == "RUNNING" {
			ls.terminal.Vprintf(ls.terminal.Yellow("\tssh %s\n", v.GetLocalIdentifier()))
		}
	}
	return nil
}

func (ls Ls) RunWorkspaces(org *entity.Organization, user *entity.User, showAll bool) error {
	if showAll {
		err := ls.ShowAllWorkspaces(org, user)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		err := ls.ShowUserWorkspaces(org, user)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func (ls Ls) RunHosts(org *entity.Organization) error {
	user, err := ls.lsStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var workspaces []entity.Workspace
	workspaces, err = ls.lsStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{UserID: user.ID})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	for _, workspace := range workspaces {
		fmt.Println(workspace.GetNodeIdentifierForVPN())
	}
	return nil
}

func displayUnjoinedProjects(t *terminal.Terminal, orgName string, projects []entity.VirtualProject) {
	if len(projects) > 0 {
		fmt.Print("\n")
		t.Vprintf("%d other projects in Org "+t.Yellow(orgName)+"\n", len(projects))
		t.Vprint(
			"NUM MEMBERS" + strings.Repeat(" ", 2+len("NUM MEMBERS")) +
				// This looks weird, but we're just giving 2*LONGEST_STATUS for the column and space between next column
				"NAME")
		for _, p := range projects {
			t.Vprintf("%d people %s %s\n", p.GetUniqueUserCount(), strings.Repeat(" ", 2*len("NUM MEMBERS")-len("people")), p.Name)
		}

		fmt.Print("\n")
		t.Vprintf(t.Green("Join a project:\n") +
			t.Yellow(fmt.Sprintf("\tbrev start %s\n", projects[0].Name)))
	} else {
		t.Vprintf("no other projects in Org "+t.Yellow(orgName)+"\n", len(projects))
		fmt.Print("\n")
		t.Vprintf(t.Green("Invite a teamate:\n") +
			t.Yellow("\tbrev invite"))
	}
}

func displayWorkspaces(t *terminal.Terminal, workspaces []entity.Workspace) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options.DrawBorder = false
	ta.Style().Options.SeparateColumns = false
	ta.Style().Options.SeparateRows = false
	ta.Style().Options.SeparateHeader = false
	ta.AppendHeader(table.Row{"NAME", "STATUS", "URL", "ID"})
	for _, w := range workspaces {
		ta.AppendRows([]table.Row{
			{w.Name, getStatusColoredText(t, w.Status), w.DNS, w.GetLocalIdentifier(), w.ID},
		})
	}
	ta.Render()
}

func getStatusColoredText(t *terminal.Terminal, status string) string {
	switch status {
	case "RUNNING":
		return t.Green(status)
	case "STARTING", "DEPLOYING", "STOPPING":
		return t.Yellow(status)
	case "FAILURE", "DELETING":
		return t.Red(status)
	default:
		return status
	}
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
