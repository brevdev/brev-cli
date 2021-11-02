package brev_api

import (
	"sync"

	"github.com/brevdev/brev-cli/pkg/files"
)

// Helper functions
func getOrgs() []Organization {
	client, _ := NewClient()
	orgs, _ := client.GetOrgs()

	return orgs
}

func getWorkspaces(orgID string) []Workspace {
	// orgID := getOrgID(orgName)

	client, _ := NewClient()
	workspaces, _ := client.GetMyWorkspaces(orgID)

	return workspaces
}

type CacheableWorkspace struct {
	OrgID      string      `json:"orgID"`
	Workspaces []Workspace `json:"workspaces"`
}

func RefreshWorkspaceCacheForActiveOrg() error {
	activeorg, err := GetActiveOrgContext()
	if err != nil {
		return err
	}
	err = WriteIndividualWorkspaceCache(activeorg.ID)
	if err != nil {
		return err
	}
	return nil
}

func WriteIndividualWorkspaceCache(orgID string) error {
	var worspaceCache []CacheableWorkspace
	path := files.GetWorkspacesCacheFilePath()
	err := files.ReadJSON(path, &worspaceCache)
	if err != nil {
		return err
	}
	wss := getWorkspaces(orgID)
	var updatedCache []CacheableWorkspace
	for _, v := range worspaceCache {
		if v.OrgID == orgID {
			v.Workspaces = wss
		}
		updatedCache = append(updatedCache, v)

	}
	err = files.OverwriteJSON(path, updatedCache)
	if err != nil {
		return err
	}
	return nil
}

func WriteCaches() error {

	orgs := getOrgs()
	path := files.GetOrgCacheFilePath()
	err := files.OverwriteJSON(path, orgs)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var worspaceCache []CacheableWorkspace
	for _, v := range orgs {
		wg.Add(1)
		go func(orgID string) {
			wss := getWorkspaces(orgID)
			worspaceCache = append(worspaceCache, CacheableWorkspace{
				OrgID: orgID, Workspaces: wss,
			})
			defer wg.Done()
		}(v.ID)
	}
	wg.Wait()
	path_ws := files.GetWorkspacesCacheFilePath()
	err2 := files.OverwriteJSON(path_ws, worspaceCache)
	if err2 != nil {
		return err2
	}
	return nil
}

func GetOrgCacheData() ([]Organization, error) {
	path := files.GetOrgCacheFilePath()
	exists, err := files.Exists(path, false)
	if err != nil {
		return nil, err
	}

	if !exists {
		err = WriteCaches()
		if err != nil {
			return nil, err
		}
	}

	var orgCache []Organization
	err = files.ReadJSON(path, &orgCache)
	if err != nil {
		return nil, err
	}
	return orgCache, nil
}

func GetWsCacheData() ([]CacheableWorkspace, error) {
	path := files.GetWorkspacesCacheFilePath()
	exists, err := files.Exists(path, false)
	if err != nil {
		return nil, err
	}

	if !exists {
		err = WriteCaches()
		if err != nil {
			return nil, err
		}
	}

	var wsCache []CacheableWorkspace
	err = files.ReadJSON(path, &wsCache)
	if err != nil {
		return nil, err
	}
	return wsCache, nil
}
