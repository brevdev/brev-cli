package brevapi

import (
	"sync"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
)

// Helper functions
func getOrgs() []Organization {
	client, err := NewClient()
	if err != nil {
		return []Organization{}
	}
	orgs, err := client.GetOrgs()
	if err != nil {
		return []Organization{}
	}
	return orgs
}

func getWorkspaces(orgID string) []Workspace {
	client, err := NewClient()
	if err != nil {
		return []Workspace{}
	}
	workspaces, err := client.GetMyWorkspaces(orgID)
	if err != nil {
		return []Workspace{}
	}
	return workspaces
}

type CacheableWorkspace struct {
	OrgID      string      `json:"orgID"`
	Workspaces []Workspace `json:"workspaces"`
}

func RefreshWorkspaceCacheForActiveOrg() error {
	activeorg, err := GetActiveOrgContext(files.AppFs)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	wss := getWorkspaces(activeorg.ID)
	err = WriteIndividualWorkspaceCache(activeorg.ID, wss)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func WriteIndividualWorkspaceCache(orgID string, wss []Workspace) error {
	var workspaceCache []CacheableWorkspace
	path := files.GetWorkspacesCacheFilePath()
	err := files.ReadJSON(path, &workspaceCache)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	var updatedCache []CacheableWorkspace
	for _, v := range workspaceCache {
		if v.OrgID == orgID {
			v.Workspaces = wss
		}
		updatedCache = append(updatedCache, v)

	}
	err = files.OverwriteJSON(path, updatedCache)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func WriteOrgCache(orgs []Organization) error {
	path := files.GetOrgCacheFilePath()
	err := files.OverwriteJSON(path, orgs)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func WriteCaches() ([]Organization, []CacheableWorkspace, error) {
	orgs := getOrgs()
	err := WriteOrgCache(orgs)
	if err != nil {
		return nil, nil, err
	}

	var wg sync.WaitGroup
	var workspaceCache []CacheableWorkspace
	for _, v := range orgs {
		wg.Add(1)
		go func(orgID string) {
			wss := getWorkspaces(orgID)
			workspaceCache = append(workspaceCache, CacheableWorkspace{
				OrgID: orgID, Workspaces: wss,
			})
			defer wg.Done()
		}(v.ID)
	}
	wg.Wait()
	path_ws := files.GetWorkspacesCacheFilePath()
	err2 := files.OverwriteJSON(path_ws, workspaceCache)
	if err2 != nil {
		return nil, nil, err2
	}
	return orgs, workspaceCache, nil
}

func GetOrgCacheData() ([]Organization, error) {
	path := files.GetOrgCacheFilePath()
	exists, err := files.Exists(path, false)
	if err != nil {
		return nil, err
	}

	if !exists {
		_, _, err = WriteCaches()
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
		_, _, err = WriteCaches()
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
