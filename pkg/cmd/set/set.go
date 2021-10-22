// Package set is for the set command
package set

import (
	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func getOrgs() []brev_api.Organization {
	client, _ := brev_api.NewClient()
	orgs, _ := client.GetOrgs()

	return orgs
}

func NewCmdSet(t *terminal.Terminal) *cobra.Command {
	var orgName string

	cmd := &cobra.Command{
		Use: "set",
		// Annotations: map[string]string{"project": ""},
		Short:   "Set active org",
		Long:    "Set your organization to view, open, create workspaces etc",
		Example: `brev set --org [org_id]`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}

			// _, err = brev_api.CheckOutsideBrevErrorMessage(t)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := set(t, orgName)
			return err
			// return nil
		},
	}

	cmd.Flags().StringVarP(&orgName, "org", "o", "", "organization name")
	cmd.MarkFlagRequired("org")
	cmd.RegisterFlagCompletionFunc("org", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		orgs := getOrgs()

		var orgNames []string
		for _, v := range orgs {
			orgNames = append(orgNames, v.Name)
		}

		return orgNames, cobra.ShellCompDirectiveNoSpace
	})

	return cmd
}

func set(t *terminal.Terminal, orgName string) error {
	orgs := getOrgs()		

	for _, v := range orgs {
		if (v.Name == orgName) {

			path := files.GetActiveOrgsPath()

			files.OverwriteJSON(path, v)
			
			t.Vprint("Organization "+ t.Green(v.Name) + " is now active.")

			return nil
		}
	}
	
	return &brev_errors.InvalidOrganizationError{}
}