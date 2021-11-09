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
	client, err := brev_api.NewCommandClient()
	if err != nil {
		panic(err)
	}
	orgs, err := client.GetOrgs()
	if err != nil {
		panic(err)
	}

	return orgs
}

func getOrgNames() []string {
	cachedOrgs, err := brev_api.GetOrgCacheData()
	if err != nil {
		return nil
	}

	// orgs  := getOrgs()
	var orgNames []string
	for _, v := range cachedOrgs {
		orgNames = append(orgNames, v.Name)
	}

	return orgNames
}

func NewCmdSet(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "set",
		Short:       "Set active org (helps with completion)",
		Long:        "Set your organization to view, open, create workspaces etc",
		Example:     `brev set [org name]`,
		Args:        cobra.MinimumNArgs(1),
		ValidArgs:   getOrgNames(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := set(t, args[0])
			return err
		},
	}

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
