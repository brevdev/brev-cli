// Package get is for the get command
package get

import (
	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func getOrgs() []brev_api.Organization {
	
	token, _ := auth.GetToken()
	brevAgent := brev_api.Agent{
		Key: token,
	}

	orgs, _ := brevAgent.GetOrgs()

	return orgs
}

func NewCmdGet(t *terminal.Terminal) *cobra.Command {
	// opts := SshOptions{}

	cmd := &cobra.Command{
		Use:         "get",
		// Annotations: map[string]string{"project": ""},
		Short:       "Get stuff",
		Long:        "Get stuff but longer.",
		Example:     `brev get [stuff]`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}

			// _, err = brev_api.CheckOutsideBrevErrorMessage(t)
			return err
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{}, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveDefault
		}}

	cmd.AddCommand(newCmdOrg(t))

	return cmd
}

func newCmdOrg(t *terminal.Terminal) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:     "organizations",
		Short:   "List your Brev organizations.",
		Long:    "List your Brev organizations.",
		Example: `  brev get organizations`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listOrgs(t)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "name of the endpoint")

	return cmd
}

func listOrgs(t *terminal.Terminal) error {
	orgs := getOrgs()
	for _, v := range orgs {
		t.Vprint(v.Name + " id:" + t.Yellow(v.Id))
	}
	return nil
}