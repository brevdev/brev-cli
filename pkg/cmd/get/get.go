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
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			
			orgs := getOrgs()
			// var orgNames []string
			for _, v := range orgs {
				// orgNames = append(orgNames, v.Name)
				t.Vprint(v.Name)

			}
			t.Vprint(t.Green("heeyyy"))
			// open(t, orgName, proj)
			return nil
		},
	}
	return cmd
}
