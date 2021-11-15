// completions provides completions
package completions

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/files"
)

type CompletionHelpers struct{}

func (c CompletionHelpers) GetWorkspaceNames() ([]string, error) {
	activeOrg, err := brevapi.GetActiveOrgContext(files.AppFs)
	if err != nil {
		return nil, err
	}

	wsCache, err := brevapi.GetWsCacheData()
	if err != nil {
		return nil, err
	}
	for _, v := range wsCache {
		if v.OrgID == activeOrg.ID {
			var workspaceNames []string
			for _, w := range v.Workspaces {
				workspaceNames = append(workspaceNames, w.Name)
			}
			return workspaceNames, nil
		}
	}
	return nil, fmt.Errorf("cache error")
}
