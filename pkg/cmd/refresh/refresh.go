// Package refresh lists workspaces in the current org
package refresh

import (
	"github.com/brevdev/brev-cli/pkg/brev_api"
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

func getWorkspaces(orgID string) []brev_api.Workspace {
	// orgID := getOrgID(orgName)

	client, _ := brev_api.NewClient()
	workspaces, _ := client.GetWorkspaces(orgID)

	return workspaces
}


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

type CacheableWorkspace struct {
	OrgID string `json:"orgID`
	Workspaces []brev_api.Workspace `json:"workspaces"`
}

// func write_individual_workspace_cache(orgID string, t *terminal.Terminal) error {
// 	var worspaceCache []CacheableWorkspace;
// 	path := files.GetWorkspacesCacheFilePath()
// 	err := files.ReadJSON(path, &worspaceCache)
// 	if err!=nil {
// 		return err
// 	}
// 	wss := getWorkspaces(orgID)
// 	for _, v := range worspaceCache {
// 		if v.OrgID == orgID {
// 			v.Workspaces = wss
// 			t.Vprintf("%d %s", len(v.Workspaces), v.OrgID)
// 			wsc := worspaceCache
// 			err := files.OverwriteJSON(path, wsc)
// 			if err!=nil {
// 				return err
// 			}
// 			return nil;
// 		}
// 	}
// 	return nil // BANANA: should this error cus it shouldn't get here???
// }

func write_caches() error {

	orgs := getOrgs()
	path := files.GetOrgCacheFilePath()
	err := files.OverwriteJSON(path, orgs)
	if err!=nil {
		return err
	}

	var worspaceCache []CacheableWorkspace;
	for _, v := range orgs {
		wss := getWorkspaces(v.ID)
		worspaceCache = append(worspaceCache, CacheableWorkspace{
			OrgID: v.ID, Workspaces: wss,
		})
	}
	path_ws := files.GetWorkspacesCacheFilePath()
	err2 := files.OverwriteJSON(path_ws, worspaceCache)
	if err2!=nil {
		return err2
	}
	return nil
}

func Get_org_cache_data() ([]brev_api.Organization,error) {
	path := files.GetOrgCacheFilePath()
	exists, err := files.Exists(path, false)
	if err != nil {
		return nil, err
	}

	if !exists {
		err = write_caches()
		if (err != nil) {
			return nil, err
		}
	}

	var orgCache []brev_api.Organization
	err = files.ReadJSON(path, &orgCache)
	if err!=nil {
		return nil, err
	}
	return orgCache, nil
}

func Get_ws_cache_data() ([]CacheableWorkspace,error) {
	path := files.GetWorkspacesCacheFilePath()
	exists, err := files.Exists(path, false)
	if err != nil {
		return nil, err
	}

	if !exists {
		err = write_caches()
		if (err != nil) {
			return nil, err
		}
	}

	var wsCache []CacheableWorkspace
	err = files.ReadJSON(path, &wsCache)
	if err!=nil {
		return nil, err
	}
	return wsCache, nil
}

func refresh(t *terminal.Terminal) error {
	// idk why this doesn't work...
	// write_individual_workspace_cache("ejmrvoj8m", t)

	err := write_caches()
	if err != nil {
		return err
	}

	t.Vprintf(t.Green("Cache has been refreshed"))

	
	
	orgs, err := Get_org_cache_data()
	if err != nil {
		return err
	}
	
	wss, err := Get_ws_cache_data()
	if err != nil {
		return err
	}

	t.Vprintf("%d" + t.Green("Orgs"), len(orgs))
	
	for _, v := range wss {
		t.Vprintf("\n%d workspaces in orgid %s", len(v.Workspaces), v.OrgID)
		if (v.OrgID == "ejmrvoj8m") {
			t.Vprint("\n\n")
			for _, w := range v.Workspaces {
				t.Vprintf("\n\t %s %s", w.DNS, w.Status)
			}
			t.Vprint("\n\n")

		}
	}



	return nil
}
