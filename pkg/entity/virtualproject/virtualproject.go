package virtualproject

import (
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/entity/generic" //nolint:typecheck // contains generic code
)

type VirtualProject struct {
	Name             string
	GitURL           string
	WorkspacesByUser map[string][]entity.Workspace
}

func NewVirtualProjects(workspaces []entity.Workspace) []VirtualProject {
	gitRepoWorkspaceMap := generic.MakeVirtualProjectMap()
	for _, w := range workspaces {
		if _, ok := gitRepoWorkspaceMap.Get(w.GitRepo); !ok {
			gitRepoWorkspaceMap.Set(w.GitRepo, make(map[string][]entity.Workspace))
		}
		m, ok := gitRepoWorkspaceMap.Get(w.GitRepo) //[w.GitRepo][w.CreatedByUserID] =
		if !ok {
			panic("no")
		}
		m[w.CreatedByUserID] = append(m[w.CreatedByUserID], w)
		gitRepoWorkspaceMap.Set(w.GitRepo, m)
	}
	var projects []VirtualProject
	for pair := gitRepoWorkspaceMap.Oldest(); pair != nil; pair = pair.Next() {
		key, ok := GetFirstKeyMap(pair.Value)
		if !ok {
			continue
		}

		if len(pair.Value[key]) == 0 {
			continue
		}

		projectName := pair.Value[key][0].Name // TODO this is the SUPER hacky unexpected behavior part
		projects = append(projects, VirtualProject{Name: projectName, GitURL: pair.Key, WorkspacesByUser: pair.Value})
	}
	return projects
}

func GetFirstKeyMap(strMap map[string][]entity.Workspace) (string, bool) {
	for k := range strMap {
		return k, true
	}
	return "", false
}

func (v VirtualProject) GetUserWorkspaces(userID string) []entity.Workspace {
	return v.WorkspacesByUser[userID]
}

func (v VirtualProject) GetUniqueUserCount() int {
	return len(v.WorkspacesByUser)
}
