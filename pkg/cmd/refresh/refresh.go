// Package refresh lists workspaces in the current org
package refresh

import (
	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)


func NewCmdRefresh(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Use: "refresh",
		// Annotations: map[string]string{"project": ""},
		Short:   "Refresh the cache",
		Long:    "Refresh Brev's cache",
		Example: `brev refresh`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}

			// _, err = brev_api.CheckOutsideBrevErrorMessage(t)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := refresh(t)
			return err
			// return nil
		},
	}

	return cmd
}


func refresh(t *terminal.Terminal) error {
	// idk why this doesn't work...
	// write_individual_workspace_cache("ejmrvoj8m", t)

	err := brev_api.Write_caches()
	if err != nil {
		return err
	}

	t.Vprintf(t.Green("Cache has been refreshed\n"))

	
	// // Prove the cached data can be read
	// orgs, err := brev_api.Get_org_cache_data()
	// if err != nil {
	// 	return err
	// }
	
	// wss, err := brev_api.Get_ws_cache_data()
	// if err != nil {
	// 	return err
	// }

	// t.Vprintf("%d" + t.Green("Orgs"), len(orgs))
	
	// for _, v := range wss {
	// 	t.Vprintf("\n%d workspaces in orgid %s", len(v.Workspaces), v.OrgID)
	// 	if (v.OrgID == "ejmrvoj8m") {
	// 		t.Vprint("\n\n")
	// 		for _, w := range v.Workspaces {
	// 			t.Vprintf("\n\t %s %s", w.DNS, w.Status)
	// 		}
	// 		t.Vprint("\n\n")

	// 	}
	// }



	return nil
}
