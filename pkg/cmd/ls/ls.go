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
	wss, err := client.GetWorkspaces(orgID)
	if err != nil {
		return nil, err
	}

	return wss, nil
}

func NewCmdLs(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "ls",
		Short:       "List workspaces",
		Long:        "List workspaces within your active org",
		Example:     `brev ls`,
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
			err := ls(t, args)
			if err != nil {
				return err
			}
			return nil
		},
	}

	return cmd
}

func ls(t *terminal.Terminal, args []string) error {
	// if len(args) > 0 && (args[0] == "orgs" || args[0] == "organizations") {
	// }
	activeorg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		return err
	}

	wss, err := getWorkspaces(activeorg.ID)
	if err != nil {
		return err
	}

	t.Vprintf("%d Workspaces in Org "+t.Yellow(activeorg.Name)+"\n", len(wss))
	for _, v := range wss {
		t.Vprint("\t• " + v.Name + " id:" + v.ID)
		t.Vprint("\t\thttps://" + v.DNS)
	}

	return nil
}
