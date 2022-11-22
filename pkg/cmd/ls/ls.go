// Package ls lists workspaces in the current org
package ls

import (
	"fmt"
	"os"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/entity/virtualproject"
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
	hello.HelloStore
}

func NewCmdLs(t *terminal.Terminal, loginLsStore LsStore, noLoginLsStore LsStore) *cobra.Command {
	var showAll bool
	var org string

	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "ls",
		Short:       "List dev environments within active org",
		Long:        "List dev environments within your active org. List all dev environments if no active org is set.",
		Example: `
  brev ls
  brev ls orgs
  brev ls --org <orgid>
		`,
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if hello.ShouldWeRunOnboardingLSStep(noLoginLsStore) && hello.ShouldWeRunOnboarding(noLoginLsStore) {
				// Getting the workspaces should go in the hello.go file but then
				// requires passing in stores and that makes it hard to use in other commands
				org, err := getOrgForRunLs(loginLsStore, org)
				if err != nil {
					return err
				}

				allWorkspaces, err := loginLsStore.GetWorkspaces(org.ID, nil)
				if err != nil {
					return err
				}

				user, err := loginLsStore.GetCurrentUser()
				if err != nil {
					return err
				}

				var myWorkspaces []entity.Workspace
				for _, v := range allWorkspaces {
					if v.CreatedByUserID == user.ID {
						myWorkspaces = append(myWorkspaces, v)
					}
				}

				err = hello.Step1(t, myWorkspaces, user, loginLsStore)
				if err != nil {
					return err
				}

			}
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		Args:      cmderrors.TransformToValidationError(cobra.MinimumNArgs(0)),
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
		breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err))
		fmt.Print(breverrors.WrapAndTrace(err))
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
			return nil, breverrors.NewValidationError(fmt.Sprintf("no org found with name %s", orgflag))
		} else if len(orgs) > 1 {
			return nil, breverrors.NewValidationError(fmt.Sprintf("more than one org found with name %s", orgflag))
		}

		org = &orgs[0]
	} else {
		var currOrg *entity.Organization
		currOrg, err := lsStore.GetActiveOrganizationOrDefault()
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if currOrg == nil {
			return nil, breverrors.NewValidationError("no orgs exist")
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
		return breverrors.NewValidationError("too many args provided")
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
		ls.terminal.Vprint(ls.terminal.Yellow("You don't have any orgs. Create one! https://console.brev.dev"))
		return nil
	}

	defaultOrg, err := ls.lsStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	ls.terminal.Vprint(ls.terminal.Yellow("Your organizations:"))
	displayOrgTable(ls.terminal, orgs, defaultOrg)
	if len(orgs) > 1 {
		fmt.Print("\n")
		ls.terminal.Vprintf(ls.terminal.Green("Switch orgs:\n"))
		notDefaultOrg := getOtherOrg(orgs, *defaultOrg)
		// TODO suggest org with max workspaces
		ls.terminal.Vprintf(ls.terminal.Yellow("\tbrev set <NAME> ex: brev set %s\n", notDefaultOrg.Name))
	}

	return nil
}

func getOtherOrg(orgs []entity.Organization, org entity.Organization) *entity.Organization {
	for _, o := range orgs {
		if org.ID != o.ID {
			return &o
		}
	}
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

func (ls Ls) ShowAllWorkspaces(org *entity.Organization, otherOrgs []entity.Organization, user *entity.User, allWorkspaces []entity.Workspace) {
	userWorkspaces := store.FilterForUserWorkspaces(allWorkspaces, user.ID)
	ls.displayWorkspacesAndHelp(org, otherOrgs, userWorkspaces, allWorkspaces)

	projects := virtualproject.NewVirtualProjects(allWorkspaces)

	var unjoinedProjects []virtualproject.VirtualProject
	for _, p := range projects {
		wks := p.GetUserWorkspaces(user.ID)
		if len(wks) == 0 {
			unjoinedProjects = append(unjoinedProjects, p)
		}
	}

	displayProjects(ls.terminal, org.Name, unjoinedProjects)
}

func (ls Ls) ShowUserWorkspaces(org *entity.Organization, otherOrgs []entity.Organization, user *entity.User, allWorkspaces []entity.Workspace) {
	userWorkspaces := store.FilterForUserWorkspaces(allWorkspaces, user.ID)

	ls.displayWorkspacesAndHelp(org, otherOrgs, userWorkspaces, allWorkspaces)
}

func (ls Ls) displayWorkspacesAndHelp(org *entity.Organization, otherOrgs []entity.Organization, userWorkspaces []entity.Workspace, allWorkspaces []entity.Workspace) {
	if len(userWorkspaces) == 0 {
		ls.terminal.Vprint(ls.terminal.Yellow("No dev environments in org %s\n", org.Name))
		if len(allWorkspaces) > 0 {
			ls.terminal.Vprintf(ls.terminal.Green("See teammates' dev environments:\n"))
			ls.terminal.Vprintf(ls.terminal.Yellow("\tbrev ls --all\n"))
		} else {
			ls.terminal.Vprintf(ls.terminal.Green("Start a new dev environment:\n"))
			ls.terminal.Vprintf(ls.terminal.Yellow("\tbrev start https://github.com/brevdev/hello-react\n"))
		}
		if len(otherOrgs) > 1 {
			ls.terminal.Vprintf(ls.terminal.Green("Switch to another org:\n"))
			// TODO suggest org with max workspaces
			ls.terminal.Vprintf(ls.terminal.Yellow(fmt.Sprintf("\tbrev set %s\n", getOtherOrg(otherOrgs, *org).Name)))
		}
	} else {
		ls.terminal.Vprintf("You have %d dev environments in Org "+ls.terminal.Yellow(org.Name)+"\n", len(userWorkspaces))
		displayWorkspacesTable(ls.terminal, userWorkspaces)

		fmt.Print("\n")

		displayLsResetBreadCrumb(ls.terminal, userWorkspaces)
		// displayLsConnectBreadCrumb(ls.terminal, userWorkspaces)

		// if !enableSSHCol {
		// 	ls.terminal.Vprintf(ls.terminal.Green("Or ssh:\n"))
		// 	for _, v := range userWorkspaces {
		// 		if v.Status == entity.Running {
		// 			ls.terminal.Vprintf(ls.terminal.Yellow("\tssh %s\n", v.GetLocalIdentifier()))
		// 		}
		// 	}
		// }
	}
}

func displayLsConnectBreadCrumb(t *terminal.Terminal, workspaces []entity.Workspace) {
	foundRunning := false
	for _, w := range workspaces {
		if w.Status == entity.Running {
			foundRunning = true
			t.Vprintf(t.Green("Connect to running dev environment:\n"))
			t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev open %s\t# brev open <NAME> -> open dev environment in preferred editor\n", w.Name)))
			t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev shell %s\t# brev shell <NAME> -> ssh into dev environment (shortcut)\n", w.Name)))
			t.Vprintf(t.Yellow(fmt.Sprintf("\tssh %s\t# ssh <SSH-NAME> -> ssh directly to dev environment\n", w.GetLocalIdentifier())))
			if enableSSHCol {
				t.Vprintf(t.Yellow("\tssh <SSH> ex: ssh %s\n", w.GetLocalIdentifier()))
			}
			break
		}
	}
	if !foundRunning && len(workspaces) > 0 {
		t.Vprintf(t.Green("Start a stopped dev environment:\n"))
		t.Vprintf(t.Yellow("\tbrev start %s # brev start <NAME> -> start stopped dev environment\n", workspaces[0].Name))
	}
}

func displayLsResetBreadCrumb(t *terminal.Terminal, workspaces []entity.Workspace) {
	foundAResettableWorkspace := false
	for _, w := range workspaces {
		if w.Status == entity.Failure || getWorkspaceDisplayStatus(w) == entity.Unhealthy {
			if !foundAResettableWorkspace {
				t.Vprintf(t.Red("Reset unhealthy or failed dev environment:\n"))
			}
			t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev reset %s\n", w.Name)))
			foundAResettableWorkspace = true
		}
	}
	if foundAResettableWorkspace {
		t.Vprintf(t.Yellow(fmt.Sprintf("If this problem persists, run the command again with the --hard flag (warning: the --hard flag will not preserve uncommitted files!) \n\n")))
	}
}

func (ls Ls) RunWorkspaces(org *entity.Organization, user *entity.User, showAll bool) error {
	allWorkspaces, err := ls.lsStore.GetWorkspaces(org.ID, nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	orgs, err := ls.lsStore.GetOrganizations(nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if showAll {
		ls.ShowAllWorkspaces(org, orgs, user, allWorkspaces)
	} else {
		ls.ShowUserWorkspaces(org, orgs, user, allWorkspaces)
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

func displayProjects(t *terminal.Terminal, orgName string, projects []virtualproject.VirtualProject) {
	if len(projects) > 0 {
		fmt.Print("\n")
		t.Vprintf("%d other projects in Org "+t.Yellow(orgName)+"\n", len(projects))
		displayProjectsTable(projects)

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

const enableSSHCol = false

func getBrevTableOptions() table.Options {
	options := table.OptionsDefault
	options.DrawBorder = false
	options.SeparateColumns = false
	options.SeparateRows = false
	options.SeparateHeader = false
	return options
}

func displayWorkspacesTable(t *terminal.Terminal, workspaces []entity.Workspace) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()
	header := table.Row{"Name", "Status", "ID", "Machine"}
	if enableSSHCol {
		header = table.Row{"Name", "Status", "SSH", "ID", "Machine"}
	}
	ta.AppendHeader(header)
	for _, w := range workspaces {
		status := getWorkspaceDisplayStatus(w)
		instanceString := getInstanceString(w)
		workspaceRow := []table.Row{{w.Name, getStatusColoredText(t, status), w.ID, instanceString}}
		if enableSSHCol {
			workspaceRow = []table.Row{{w.Name, getStatusColoredText(t, status), w.GetLocalIdentifier(), w.ID, instanceString}}
		}
		ta.AppendRows(workspaceRow)
	}
	ta.Render()
}

func getClassIDString(classID string) string {
	// switch statement on class ID
	switch classID {
	case "2x2":
		return "2 cpu | 2 gb ram"
	case "2x4":
		return "2 cpu | 4 gb ram"
	case "2x8":
		return "2 cpu | 8 gb ram"
	case "4x16":
		return "4 cpu | 16 gb ram"
	case "8x32":
		return "8 cpu | 32 gb ram"
	case "16x32":
		return "16 cpu | 32 gb ram"
	default:
		return classID

	}
}

func getInstanceString(w entity.Workspace) string {
	var instanceString string
	if w.WorkspaceClassID != "" {
		instanceString = getClassIDString(w.WorkspaceClassID)
	} else {
		instanceString = w.InstanceType + " (gpu)"
	}
	return instanceString
}

func getWorkspaceDisplayStatus(w entity.Workspace) string {
	status := w.Status
	if w.Status == entity.Running && w.HealthStatus == entity.Unhealthy {
		status = w.HealthStatus
	}
	return status
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

func displayProjectsTable(projects []virtualproject.VirtualProject) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()
	header := table.Row{"NAME", "MEMBERS"}
	ta.AppendHeader(header)
	for _, p := range projects {
		workspaceRow := []table.Row{{p.Name, p.GetUniqueUserCount()}}
		ta.AppendRows(workspaceRow)
	}
	ta.Render()
}

func getStatusColoredText(t *terminal.Terminal, status string) string {
	switch status {
	case entity.Running:
		return t.Green(status)
	case entity.Starting, entity.Deploying, entity.Stopping:
		return t.Yellow(status)
	case entity.Failure, entity.Deleting, entity.Unhealthy:
		return t.Red(status)
	default:
		return status
	}
}
