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
		Annotations: map[string]string{"context": ""},
		Use:         "set",
		Short:       "Set active org",
		Long:        "Set your organization to view, open, create workspaces etc",
		Example:     `brev set --org [org name]`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := set(t, orgName)
			return err
		},
	}

	cmd.Flags().StringVarP(&orgName, "org", "o", "", "organization name")
	cmd.MarkFlagRequired("org")
	cmd.RegisterFlagCompletionFunc("org", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Note: we're only using the cached data in the shell completion.
		// BANANA: run this by Alec. For sake of speed, we might want the actual cmd to only use cache too
		orgs, err := brev_api.GetOrgCacheData()
		if err != nil {
			t.Errprint(err, "")
		}

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
		if v.Name == orgName {
			path := files.GetActiveOrgsPath()

			err := files.OverwriteJSON(path, v)
			if err != nil {
				return err
			}

			t.Vprint("Organization " + t.Green(v.Name) + " is now active.")

			return nil
		}
	}

	return &brev_errors.InvalidOrganizationError{}
}
