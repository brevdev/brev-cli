// Package ls lists workspaces in the current org
package ls

import (
	"errors"
	"strings"
	"sync"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// Helper functions.
func getMe() brev_api.User {
	client, err := brev_api.NewCommandClient()
	if err != nil {
		panic(err)
	}
	user, err := client.GetMe()
	if err != nil {
		panic(err)
	}
	return *user
}

func getOrgs() ([]brev_api.Organization, error) {
	client, err := brev_api.NewCommandClient()
	if err != nil {
		return nil, err
	}
	orgs, err := client.GetOrgs()
	if err != nil {
		return nil, err
	}
	return orgs, nil
}

func GetAllWorkspaces(orgID string) ([]brev_api.Workspace, error) {
	client, err := brev_api.NewCommandClient()
	if err != nil {
		return nil, err
	}
	wss, err := client.GetWorkspaces(orgID)
	if err != nil {
		return nil, err
	}

	return wss, nil
}

func NewCmdLs(t *terminal.Terminal) *cobra.Command {
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
				return err
			}

			return nil
		},
		Args:      cobra.MinimumNArgs(0),
		ValidArgs: []string{"orgs", "workspaces"},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := ls(t, args, org)
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org)")
	cmd.RegisterFlagCompletionFunc("org", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return brev_api.GetOrgNames(), cobra.ShellCompDirectiveNoSpace
	})

	return cmd
}

func ls(t *terminal.Terminal, args []string, orgflag string) error {
	// If anyone mispells orgs/org/organizations we should just do it
	if len(args) > 0 && (strings.Contains(args[0], "org")) {
		orgs, err := getOrgs()
		if err != nil {
			return err
		}
		if len(orgs) == 0 {
			t.Vprint(t.Yellow("You don't have any orgs. Create one!"))
			return nil
		}

		activeorg, err := brev_api.GetActiveOrgContext(files.AppFs)
		if err != nil {
			t.Vprint(t.Yellow("Your organizations:"))
			err = printOrgTableWithoutActiveOrg(t, orgs)
			return nil
		}
		t.Vprint(t.Yellow("Your organizations:"))
		err = printOrgTable(t, orgs, *activeorg)
		if err != nil {
			return err
		}

		return nil
	}

	if orgflag != "" {
		org, err := brev_api.GetOrgFromName(orgflag)
		if err != nil {
			return err
		}
		joined, _, err := fetchWorkspacesAndPrintTable(t, org)
		if err != nil {
			return err
		}
		if len(joined) > 0 {
			t.Vprintf(t.Green("\n\nTo connect to your machine, make sure to Brev on:") +
				t.Yellow("\n\t$ brev on\n"))
		}
		return nil
	}

	activeorg, err := brev_api.GetActiveOrgContext(files.AppFs)
	// activeOrgFoundErr := brev_errors.ActiveOrgFileNotFound{}
	// if errors.Is(err, &activeOrgFoundErr) {
	if err != nil {

		activeOrgFoundErr := brev_errors.ActiveOrgFileNotFound{}
		if errors.Is(err, &activeOrgFoundErr) {
			orgs, err := getOrgs()
			if err != nil {
				return err
			}
			if len(orgs) == 0 {
				t.Vprint(t.Yellow("You don't have any orgs or workspaces. Create an org to get started!"))
				return nil
			}
			var wg sync.WaitGroup

			for _, o := range orgs {
				wg.Add(1)
				err := make(chan error)

				o := o
				go func(t *terminal.Terminal, org *brev_api.Organization) {
					_, _, err1 := fetchWorkspacesAndPrintTable(t, org)
					err <- err1
					defer wg.Done()
				}(t, &o)

				err_ := <-err
				if err_ != nil {
					return err_
				}

			}
			wg.Wait()
			t.Vprint(t.Yellow("\nYou don't have any active org set. Run 'brev set <orgname>' to set one."))

			return nil
		} else {
			return err
		}
	}

	joined, _, err := fetchWorkspacesAndPrintTable(t, activeorg)
	if err != nil {
		return err
	}
	if len(joined) > 0 {
		t.Vprintf(t.Green("\n\nTo connect to your machine, make sure to Brev on:") +
			t.Yellow("\n\t$ brev on\n"))
	}

	if err != nil {
		return err
	}

	return nil
}

func fetchWorkspacesAndPrintTable(t *terminal.Terminal, org *brev_api.Organization) ([]brev_api.Workspace, []brev_api.Workspace, error) {
	wss, err := GetAllWorkspaces(org.ID)
	if err != nil {
		return nil, nil, err
	}
	if len(wss) == 0 {
		t.Vprint(t.Yellow("You don't have any workspaces in org %s.", org.Name))
		return nil, nil, nil
	}
	o := org
	joined, unjoined, err := printWorkspaceTable(t, wss, *o)
	if err != nil {
		return nil, nil, err
	}
	return joined, unjoined, nil
}

func truncateString(s string, delimterCount int) string {
	if len(s) <= delimterCount {
		return s
	} else {
		return s[:delimterCount]
	}
}

func printWorkspaceTable(t *terminal.Terminal, workspaces []brev_api.Workspace, activeorg brev_api.Organization) ([]brev_api.Workspace, []brev_api.Workspace, error) {
	unjoinedWorkspaces, joinedWorkspaces := GetSortedUserWorkspaces(workspaces)

	DELIMETER := 40
	LONGEST_STATUS := len("DEPLOYING") // longest name for a workspace status, used for table formatting
	if len(joinedWorkspaces) > 0 {
		t.Vprintf("\nYou have %d workspaces in Org "+t.Yellow(activeorg.Name)+"\n", len(joinedWorkspaces))
		t.Vprint(
			"NAME" + strings.Repeat(" ", DELIMETER+1-len("NAME")) +
				// This looks weird, but we're just giving 2*LONGEST_STATUS for the column and space between next column
				"STATUS" + strings.Repeat(" ", LONGEST_STATUS+1+LONGEST_STATUS-len("STATUS")) +
				"ID" + strings.Repeat(" ", len(joinedWorkspaces[0].ID)+5-len("ID")) +
				"URL")
		for _, v := range joinedWorkspaces {
			t.Vprint(
				truncateString(v.Name, DELIMETER) + strings.Repeat(" ", DELIMETER-len(truncateString(v.Name, DELIMETER))) + " " +
					getStatusColoredText(t, v.Status) + strings.Repeat(" ", LONGEST_STATUS+LONGEST_STATUS-len(v.Status)) + " " +
					v.ID + strings.Repeat(" ", 5) +
					v.DNS)
		}
	}

	return joinedWorkspaces, unjoinedWorkspaces, nil
}

func GetSortedUserWorkspaces(workspaces []brev_api.Workspace) ([]brev_api.Workspace, []brev_api.Workspace) {
	me := getMe()

	var unjoinedWorkspaces []brev_api.Workspace
	var joinedWorkspaces []brev_api.Workspace

	for _, v := range workspaces {
		if v.CreatedByUserID == me.Id {
			joinedWorkspaces = append(joinedWorkspaces, v)
		} else {
			unjoinedWorkspaces = append(unjoinedWorkspaces, v)
		}
	}
	return unjoinedWorkspaces, joinedWorkspaces
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

func printOrgTable(t *terminal.Terminal, organizations []brev_api.Organization, activeorg brev_api.Organization) error {
	ID_LEN := 9
	if len(organizations) > 0 {
		t.Vprint("  ID" + strings.Repeat(" ", ID_LEN+1-len("ID")) + "NAME")
		for _, v := range organizations {
			if activeorg.ID == v.ID {
				t.Vprint(t.Green("* " + truncateString(v.ID, ID_LEN) + strings.Repeat(" ", ID_LEN-len(truncateString(v.ID, ID_LEN))) + " " + v.Name))
			} else {
				t.Vprint("  " + truncateString(v.ID, ID_LEN) + strings.Repeat(" ", ID_LEN-len(truncateString(v.ID, ID_LEN))) + " " + v.Name)
			}
		}
	}
	return nil
}

func printOrgTableWithoutActiveOrg(t *terminal.Terminal, organizations []brev_api.Organization) error {
	ID_LEN := 9
	if len(organizations) > 0 {
		t.Vprint("ID" + strings.Repeat(" ", ID_LEN+1-len("ID")) + "NAME")
		for _, v := range organizations {
			t.Vprint(truncateString(v.ID, ID_LEN) + strings.Repeat(" ", ID_LEN-len(truncateString(v.ID, ID_LEN))) + " " + v.Name)
		}
	}
	t.Vprint(t.Yellow("\nYou haven't set an active org. Use 'brev set [org_name]' to set one.\n"))
	return nil
}
