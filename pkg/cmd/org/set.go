package org

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/server"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func NewCmdOrgSet(t *terminal.Terminal, orgcmdStore OrgCmdStore, noorgcmdStore OrgCmdStore) *cobra.Command {
	var showAll bool
	var org string

	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "set",
		Short:       "Set active org",
		Long:        "Set your organization to view, open, create workspaces etc",
		Example: `
  brev org set <NAME>
		`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		Args: cobra.MinimumNArgs(1),
		// ValidArgs: []string{"new", "ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := set(args[0], orgcmdStore, t)
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

	cmd.Flags().BoolVar(&showAll, "all", false, "show all workspaces in org")

	return cmd
}

func set(orgName string, setStore OrgCmdStore, t *terminal.Terminal) error {
	workspaceID, err := setStore.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println(workspaceID)

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
	t.Vprintf("Org %s is now active 🤙\n", t.Green(org.Name))
	user, err := setStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if featureflag.IsAdmin(user.GlobalUserType) {
		sock := setStore.GetServerSockFile()
		c, err := server.NewClient(sock)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = c.ConfigureVPN()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	// Print workspaces within org

	return nil
}
