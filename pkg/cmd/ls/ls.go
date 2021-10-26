// Package ls lists workspaces in the current org
package ls

import (
	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func getWorkspaces(orgID string) ([]brev_api.Workspace, error) {
	client, err := brev_api.NewClient()
	// wss = workspaces, is that a bad name?
	wss, err := client.GetWorkspaces(orgID)
	if err != nil {
		return nil, err
	}

	return wss, nil
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

	wss := getWorkspaces(activeorg.ID)

	t.Vprintf("%d Workspaces in Org "+t.Yellow(activeorg.Name)+"\n", len(wss))
	for _, v := range wss {
		t.Vprint("\t• " + v.Name + " id:" + v.ID)
		t.Vprint("\t\t" + v.DNS)
	}

	return nil
}
