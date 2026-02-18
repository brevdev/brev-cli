// Package ls lists workspaces in the current org
package ls

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	cmdutil "github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/config"
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
	var jsonOutput bool

	cmd := &cobra.Command{
		Annotations: map[string]string{"workspace": ""},
		Use:         "ls",
		Aliases:     []string{"list"},
		Short:       "List instances within active org",
		Long: `List instances within your active org. List all instances if no active org is set.

When stdout is piped, outputs instance names only (one per line) for easy chaining
with other commands like stop, start, or delete.`,
		Example: `
  brev ls
  brev ls --json
  brev ls | grep running | brev stop
  brev ls orgs
  brev ls orgs --json
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
					return breverrors.WrapAndTrace(err)
				}

				user, err := loginLsStore.GetCurrentUser()
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}

				var myWorkspaces []entity.Workspace
				for _, v := range allWorkspaces {
					if v.CreatedByUserID == user.ID {
						myWorkspaces = append(myWorkspaces, v)
					}
				}

				err = hello.Step1(t, myWorkspaces, user, loginLsStore)
				if err != nil {
					return breverrors.WrapAndTrace(err)
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
			// Auto-switch to names-only output when piped (for chaining with stop/start/delete)
			piped := cmdutil.IsStdoutPiped()

			err := RunLs(t, loginLsStore, args, org, showAll, jsonOutput, piped)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			// Call analytics for ls (skip when piped to avoid polluting output)
			if !piped && !jsonOutput {
				trackLsAnalytics(loginLsStore)
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
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")

	return cmd
}

// trackLsAnalytics sends analytics event for ls command
func trackLsAnalytics(store LsStore) {
	userID := ""
	user, err := store.GetCurrentUser()
	if err == nil {
		userID = user.ID
	}
	data := analytics.EventData{
		EventName: "Brev ls",
		UserID:    userID,
	}
	_ = analytics.TrackEvent(data)
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

func RunLs(t *terminal.Terminal, lsStore LsStore, args []string, orgflag string, showAll bool, jsonOutput bool, piped bool) error {
	ls := NewLs(lsStore, t, jsonOutput, piped)
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
	lsStore    LsStore
	terminal   *terminal.Terminal
	jsonOutput bool
	piped      bool
}

func NewLs(lsStore LsStore, terminal *terminal.Terminal, jsonOutput bool, piped bool) *Ls {
	return &Ls{
		lsStore:    lsStore,
		terminal:   terminal,
		jsonOutput: jsonOutput,
		piped:      piped,
	}
}

// OrgInfo represents organization data for JSON output
type OrgInfo struct {
	Name     string `json:"name"`
	ID       string `json:"id"`
	IsActive bool   `json:"is_active"`
}

func (ls Ls) RunOrgs() error {
	orgs, err := ls.lsStore.GetOrganizations(nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		if ls.jsonOutput {
			fmt.Println("[]")
			return nil
		}
		if ls.piped {
			return nil
		}
		ls.terminal.Vprint(ls.terminal.Yellow(fmt.Sprintf("You don't have any orgs. Create one! %s", config.GlobalConfig.GetConsoleURL())))
		return nil
	}

	defaultOrg, err := ls.lsStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Handle JSON output
	if ls.jsonOutput {
		return ls.outputOrgsJSON(orgs, defaultOrg)
	}

	// Handle piped output - clean table without colors
	if ls.piped {
		displayOrgTablePlain(orgs, defaultOrg)
		return nil
	}

	// Standard table output
	ls.terminal.Vprint(ls.terminal.Yellow("Your organizations:"))
	displayOrgTable(ls.terminal, orgs, defaultOrg)
	if len(orgs) > 1 {
		fmt.Print("\n")
		ls.terminal.Vprintf("%s", ls.terminal.Green("Switch orgs:\n"))
		notDefaultOrg := getOtherOrg(orgs, *defaultOrg)
		// TODO suggest org with max workspaces
		ls.terminal.Vprintf("%s", ls.terminal.Yellow("\tbrev set <NAME> ex: brev set %s\n", notDefaultOrg.Name))
	}

	return nil
}

func (ls Ls) outputOrgsJSON(orgs []entity.Organization, defaultOrg *entity.Organization) error {
	var infos []OrgInfo
	for _, o := range orgs {
		infos = append(infos, OrgInfo{
			Name:     o.Name,
			ID:       o.ID,
			IsActive: defaultOrg != nil && o.ID == defaultOrg.ID,
		})
	}
	output, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println(string(output))
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
		ls.terminal.Vprint(ls.terminal.Yellow("No instances in org %s\n", org.Name))
		if len(allWorkspaces) > 0 {
			ls.terminal.Vprintf("%s", ls.terminal.Green("See teammates' instances:\n"))
			ls.terminal.Vprintf("%s", ls.terminal.Yellow("\tbrev ls --all\n"))
		} else {
			ls.terminal.Vprintf("%s", ls.terminal.Green("Start a new instance:\n"))
		}
		if len(otherOrgs) > 1 {
			ls.terminal.Vprintf("%s", ls.terminal.Green("Switch to another org:\n"))
			// TODO suggest org with max workspaces
			ls.terminal.Vprintf("%s", ls.terminal.Yellow(fmt.Sprintf("\tbrev set %s\n", getOtherOrg(otherOrgs, *org).Name)))
		}
	} else {
		ls.terminal.Vprintf("You have %d instances in Org %s\n", len(userWorkspaces), ls.terminal.Yellow(org.Name))
		displayWorkspacesTable(ls.terminal, userWorkspaces)

		fmt.Print("\n")

		displayLsResetBreadCrumb(ls.terminal, userWorkspaces)
		// displayLsConnectBreadCrumb(ls.terminal, userWorkspaces)
	}
}

func displayLsResetBreadCrumb(t *terminal.Terminal, workspaces []entity.Workspace) {
	foundAResettableWorkspace := false
	for _, w := range workspaces {
		if w.Status == entity.Failure || getWorkspaceDisplayStatus(w) == entity.Unhealthy {
			if !foundAResettableWorkspace {
				t.Vprintf("%s", t.Red("Reset unhealthy or failed instance:\n"))
			}
			t.Vprintf("%s", t.Yellow(fmt.Sprintf("\tbrev reset %s\n", w.Name)))
			foundAResettableWorkspace = true
		}
	}
	if foundAResettableWorkspace {
		t.Vprintf("%s", t.Yellow("If this problem persists, run the command again with the --hard flag (warning: the --hard flag will not preserve uncommitted files!) \n\n"))
	}
}

func (ls Ls) RunWorkspaces(org *entity.Organization, user *entity.User, showAll bool) error {
	allWorkspaces, err := ls.lsStore.GetWorkspaces(org.ID, nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Determine which workspaces to show
	var workspacesToShow []entity.Workspace
	if showAll {
		workspacesToShow = allWorkspaces
	} else {
		workspacesToShow = store.FilterForUserWorkspaces(allWorkspaces, user.ID)
	}

	// Handle JSON output
	if ls.jsonOutput {
		return ls.outputWorkspacesJSON(workspacesToShow)
	}

	// Handle piped output - clean table without colors or extra text
	// Enables: brev ls | grep RUNNING | awk '{print $1}' | brev stop
	if ls.piped {
		displayWorkspacesTablePlain(workspacesToShow)
		return nil
	}

	// Standard table output with colors and help text
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

// WorkspaceInfo represents workspace data for JSON output
type WorkspaceInfo struct {
	Name         string `json:"name"`
	ID           string `json:"id"`
	Status       string `json:"status"`
	BuildStatus  string `json:"build_status"`
	ShellStatus  string `json:"shell_status"`
	HealthStatus string `json:"health_status"`
	InstanceType string `json:"instance_type"`
	InstanceKind string `json:"instance_kind"`
}

// getInstanceTypeAndKind returns the instance type and kind (gpu/cpu)
func getInstanceTypeAndKind(w entity.Workspace) (string, string) {
	if w.InstanceType != "" {
		return w.InstanceType, "gpu"
	}
	if w.WorkspaceClassID != "" {
		return w.WorkspaceClassID, "cpu"
	}
	return "", ""
}

func (ls Ls) outputWorkspacesJSON(workspaces []entity.Workspace) error {
	var infos []WorkspaceInfo
	for _, w := range workspaces {
		instanceType, instanceKind := getInstanceTypeAndKind(w)
		infos = append(infos, WorkspaceInfo{
			Name:         w.Name,
			ID:           w.ID,
			Status:       getWorkspaceDisplayStatus(w),
			BuildStatus:  string(w.VerbBuildStatus),
			ShellStatus:  getShellDisplayStatus(w),
			HealthStatus: w.HealthStatus,
			InstanceType: instanceType,
			InstanceKind: instanceKind,
		})
	}
	output, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println(string(output))
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
		t.Vprintf("%d other projects in Org %s\n", len(projects), t.Yellow(orgName))
		displayProjectsTable(projects)

		fmt.Print("\n")
		t.Vprintf("%s", t.Green("Join a project:\n")+
			t.Yellow(fmt.Sprintf("\tbrev start %s\n", projects[0].Name)))
	} else {
		t.Vprintf("no other projects in Org %s\n", t.Yellow(orgName))
		fmt.Print("\n")
		t.Vprintf("%s", t.Green("Invite a teamate:\n")+
			t.Yellow("\tbrev invite"))
	}
}

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
	header := table.Row{"Name", "Status", "Build", "Shell", "ID", "Machine"}
	ta.AppendHeader(header)
	for _, w := range workspaces {
		status := getWorkspaceDisplayStatus(w)
		instanceString := cmdutil.GetInstanceString(w)
		workspaceRow := []table.Row{{w.Name, getStatusColoredText(t, status), getStatusColoredText(t, string(w.VerbBuildStatus)), getStatusColoredText(t, getShellDisplayStatus(w)), w.ID, instanceString}}
		ta.AppendRows(workspaceRow)
	}
	ta.Render()
}

// displayWorkspacesTablePlain outputs a clean table without colors for piping
// Enables: brev ls | grep RUNNING | awk '{print $1}' | brev stop
func displayWorkspacesTablePlain(workspaces []entity.Workspace) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()
	header := table.Row{"NAME", "STATUS", "BUILD", "SHELL", "ID", "MACHINE"}
	ta.AppendHeader(header)
	for _, w := range workspaces {
		status := getWorkspaceDisplayStatus(w)
		instanceString := cmdutil.GetInstanceString(w)
		workspaceRow := []table.Row{{w.Name, status, string(w.VerbBuildStatus), getShellDisplayStatus(w), w.ID, instanceString}}
		ta.AppendRows(workspaceRow)
	}
	ta.Render()
}

func getShellDisplayStatus(w entity.Workspace) string {
	status := entity.NotReady
	if w.Status == entity.Running && w.VerbBuildStatus == entity.Completed {
		status = entity.Ready
	}
	return status
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

// displayOrgTablePlain outputs a clean table without colors for piping
// Enables: brev ls orgs | grep myorg | awk '{print $1}'
func displayOrgTablePlain(orgs []entity.Organization, currentOrg *entity.Organization) {
	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()
	header := table.Row{"NAME", "ID"}
	ta.AppendHeader(header)
	for _, o := range orgs {
		activeMarker := ""
		if currentOrg != nil && o.ID == currentOrg.ID {
			activeMarker = "* "
		}
		ta.AppendRows([]table.Row{{activeMarker + o.Name, o.ID}})
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
	case entity.Running, entity.Ready, string(entity.Completed):
		return t.Green(status)
	case entity.Starting, entity.Deploying, entity.Stopping, string(entity.Building), string(entity.Pending):
		return t.Yellow(status)
	case entity.Failure, entity.Deleting, entity.Unhealthy, string(entity.CreateFailed):
		return t.Red(status)
	default:
		return status
	}
}
