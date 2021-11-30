// Package ls lists workspaces in the current org
package ls

import (
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type LsStore interface {
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]brevapi.Workspace, error)
	GetActiveOrganizationOrDefault() (*brevapi.Organization, error)
	GetCurrentUser() (*brevapi.User, error)
	GetWorkspace(workspaceID string) (*brevapi.Workspace, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]brevapi.Organization, error)
}

func NewCmdLs(t *terminal.Terminal, loginLsStore LsStore, noLoginLsStore LsStore) *cobra.Command {
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
			err := RunLs(t, loginLsStore, args, org)
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

	return cmd
}

func RunLs(t *terminal.Terminal, lsStore LsStore, args []string, orgflag string) error {
	ls := NewLs(lsStore, t)
	if len(args) == 1 && (strings.Contains(args[0], "org")) { // handle org, orgs, and organization(s)
		err := ls.RunOrgs()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		return nil
	} else if len(args) > 1 {
		return fmt.Errorf("too many args provided")
	}

	var org *brevapi.Organization
	if orgflag != "" {
		orgs, err := lsStore.GetOrganizations(&store.GetOrganizationsOptions{Name: orgflag})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if len(orgs) == 0 {
			return fmt.Errorf("no org found with name %s", orgflag)
		} else if len(orgs) > 1 {
			return fmt.Errorf("more than one org found with name %s", orgflag)
		}

		org = &orgs[0]
	} else {
		currOrg, err := lsStore.GetActiveOrganizationOrDefault()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if currOrg == nil {
			return fmt.Errorf("no orgs exist")
		}
		org = currOrg
	}

	err := ls.RunWorkspaces(org)
	if err != nil {
		return breverrors.WrapAndTrace(err)
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

func (ls Ls) RunWorkspaces(org *brevapi.Organization) error {
	user, err := ls.lsStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspaces, err := ls.lsStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{UserID: user.ID})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(workspaces) == 0 {
		ls.terminal.Vprint(ls.terminal.Yellow("You don't have any workspaces in org %s.", org.Name))
		return nil
	}

	displayOrgWorkspaces(ls.terminal, workspaces, org)

	ls.terminal.Vprintf(ls.terminal.Green("\n\nTo connect to your machine:") +
		ls.terminal.Yellow("\n\t$ brev up\n"))
	return nil
}

func displayOrgWorkspaces(t *terminal.Terminal, workspaces []brevapi.Workspace, org *brevapi.Organization) {
	delimeter := 40
	longestStatus := len("DEPLOYING") // longest name for a workspace status, used for table formatting
	if len(workspaces) > 0 {
		t.Vprintf("\nYou have %d workspaces in Org "+t.Yellow(org.Name)+"\n", len(workspaces))
		t.Vprint(
			"NAME" + strings.Repeat(" ", delimeter+1-len("NAME")) +
				// This looks weird, but we're just giving 2*LONGEST_STATUS for the column and space between next column
				"STATUS" + strings.Repeat(" ", longestStatus+1+longestStatus-len("STATUS")) +
				"ID" + strings.Repeat(" ", len(workspaces[0].ID)+5-len("ID")) +
				"URL")
		for _, v := range workspaces {
			t.Vprint(
				truncateString(v.Name, delimeter) + strings.Repeat(" ", delimeter-len(truncateString(v.Name, delimeter))) + " " +
					getStatusColoredText(t, v.Status) + strings.Repeat(" ", longestStatus+longestStatus-len(v.Status)) + " " +
					v.ID + strings.Repeat(" ", 5) +
					v.DNS)
		}
	}
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

func displayOrgs(t *terminal.Terminal, organizations []brevapi.Organization, defaultOrg *brevapi.Organization) {
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
