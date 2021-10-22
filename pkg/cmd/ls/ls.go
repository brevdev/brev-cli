// Package set is for the set command
package ls

import (
	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func getOrgs() []brev_api.Organization {
	client, _ := brev_api.NewClient()
	orgs, _ := client.GetOrgs()

	return orgs
}

func NewCmdLs(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Use: "ls",
		// Annotations: map[string]string{"project": ""},
		Short:   "List workspaces",
		Long:    "List workspaces within your active org",
		Example: `brev ls`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}

			// _, err = brev_api.CheckOutsideBrevErrorMessage(t)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := ls(t)
			return err
			// return nil
		},
	}

	return cmd
}

func ls(t *terminal.Terminal) error {
	activeorg, err := brev_api.GetActiveOrgContext()

	if err != nil {
		return err
	}

	t.Vprint(activeorg.Name)

	return nil
}