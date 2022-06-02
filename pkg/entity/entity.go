package entity

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// IdeConfig {
//     VsCode: {
//         Extensions: []
//     }
// }

type VSCodeExtensionMetadata struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Version     string `json:"version"`
	Publisher   string `json:"publisher"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
}

type VsCode struct {
	Extensions []VSCodeExtensionMetadata `json:"extensions"`
}

type IdeConfig struct {
	VsCode VsCode `json:"vscode"`
}

type UpdateUser struct {
	Username          string                 `json:"username,omitempty"`
	Name              string                 `json:"name,omitempty"`
	Email             string                 `json:"email,omitempty"`
	BaseWorkspaceRepo string                 `json:"baseWorkspaceRepo,omitempty"`
	OnboardingStatus  map[string]interface{} `json:"onboardingData,omitempty"` // todo fix inconsitency
	IdeConfig         IdeConfig              `json:"ideConfig,omitempty"`
}

type User struct {
	ID                string                 `json:"id"`
	PublicKey         string                 `json:"publicKey,omitempty"`
	Username          string                 `json:"username"`
	Name              string                 `json:"name"`
	Email             string                 `json:"email"`
	WorkspacePassword string                 `json:"workspacePassword"`
	BaseWorkspaceRepo string                 `json:"baseWorkspaceRepo"`
	GlobalUserType    string                 `json:"globalUserType"`
	IdeConfig         IdeConfig              `json:"IdeConfig,omitempty"`
	OnboardingStatus  map[string]interface{} `json:"onboardingData"` // todo fix inconsitency
}

type UserKeys struct {
	PrivateKey      string               `json:"privateKey"`
	PublicKey       string               `json:"publicKey"`
	WorkspaceGroups []WorkspaceGroupKeys `json:"workspaceGroups"`
}

type WorkspaceGroupKeys struct {
	GroupID string `json:"groupId"`
	Cert    string `json:"cert"`
	CA      string `json:"ca"`
	APIURL  string `json:"apiUrl"`
}

func (u UserKeys) GetWorkspaceGroupKeysByGroupID(groupID string) (*WorkspaceGroupKeys, error) {
	for _, wgk := range u.WorkspaceGroups {
		if wgk.GroupID == groupID {
			return &wgk, nil
		}
	}
	return nil, fmt.Errorf("group id %s not found", groupID)
}

type Organization struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	UserNetworkID string `json:"userNetworkId"`
}

type WorkspaceMetaData struct {
	PodName       string `json:"podName"`
	NamespaceName string `json:"namespaceName"`
}

func (w WorkspaceMetaData) GetPodName() string {
	return w.PodName
}

func (w WorkspaceMetaData) GetNamespaceName() string {
	return w.NamespaceName
}

type WorkspaceWithMeta struct {
	WorkspaceMetaData
	Workspace
}

func WorkspacesWithMetaToWorkspaces(wms []WorkspaceWithMeta) []Workspace {
	ws := []Workspace{}
	for _, wm := range wms {
		ws = append(ws, wm.Workspace)
	}
	return ws
}

type Application struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Port         int    `json:"port"`
	StartCommand string `json:"startCommand"`
	Version      string `json:"version"`
}

type RequestCreateWorkspace struct {
	Name                 string        `json:"name"`
	WorkspaceGroupID     string        `json:"workspaceGroupId"`
	WorkspaceClassID     string        `json:"workspaceClassId"`
	GitRepo              string        `json:"gitRepo"`
	IsStoppable          bool          `json:"isStoppable"`
	WorkspaceTemplateID  string        `json:"workspaceTemplateId"`
	PrimaryApplicationID string        `json:"primaryApplicationId"`
	Applications         []Application `json:"applications"`
}

type WorkspaceLocalID string

const (
	WorkspaceRunningStatus  = "RUNNING"
	WorkspaceStartingStatus = "STARTING"
	WorkspaceStoppingStatus = "STOPPING"
	WorkspaceDeletingStatus = "DELETING"
)

type Workspace struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	WorkspaceGroupID string `json:"workspaceGroupId"`
	OrganizationID   string `json:"organizationId"`
	// WorkspaceClassID is resources, like "2x8"
	WorkspaceClassID  string            `json:"workspaceClassId"`
	CreatedByUserID   string            `json:"createdByUserId"`
	DNS               string            `json:"dns"`
	Status            string            `json:"status"`
	Password          string            `json:"password"`
	GitRepo           string            `json:"gitRepo"`
	Version           string            `json:"version"`
	WorkspaceTemplate WorkspaceTemplate `json:"workspaceTemplate"`
	NetworkID         string            `json:"networkId"`
	// The below are other fields that might not be needed yet so commented out
	// PrimaryApplicationId         string `json:"primaryApplicationId,omitempty"`
	// LastOnlineAt         string `json:"lastOnlineAt,omitempty"`
	// HealthStatus         string `json:"healthStatus,omitempty"`
	// CreatedAt         string `json:"createdAt,omitempty"`
	// UpdatedAt         string `json:"updatedAt,omitempty"`
	// Version         string `json:"version,omitempty"`
	// IsStoppable         string `json:"isStoppable,omitempty"`
	// StatusMessage         string `json:"statusMessage,omitempty"`
}

type WorkspaceTemplate struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	RegistryURI string `json:"registryUri"`
	Image       string `json:"image"`
	Public      bool   `json:"public"`
	Port        int    `json:"port"`
}

func (w Workspace) GetProjectFolderPath() string {
	var folderName string
	if len(w.GitRepo) > 0 {
		splitBySlash := strings.Split(w.GitRepo, "/")[1]
		repoPath := strings.Split(splitBySlash, ".git")[0]
		folderName = repoPath
	} else {
		folderName = w.Name
	}

	return filepath.Join("/home/brev/workspace/", folderName)
}

const featureSimpleNames = false

func (w Workspace) GetLocalIdentifier() WorkspaceLocalID {
	if featureSimpleNames {
		return w.createSimpleName()
	} else {
		return w.createUniqueReadableName()
	}
}

func (w Workspace) createUniqueReadableName() WorkspaceLocalID {
	return WorkspaceLocalID(fmt.Sprintf("%s-%s", CleanSubdomain(w.Name), MakeIDSuffix(w.ID)))
}

func MakeIDSuffix(id string) string {
	return id[len(id)-4:]
}

var (
	whitespaceCharPattern = regexp.MustCompile(`\s+`)
	invalidCharPattern    = regexp.MustCompile(`[^a-z0-9-]`)
)

// lowercase, replace whitespace with '-', remove all [^a-z0-9-], trim '-' front and back
func CleanSubdomain(in string) string {
	lowered := strings.ToLower(in)
	whitespaceReplacedWithDash := whitespaceCharPattern.ReplaceAllString(lowered, "-")
	removedInvalidChars := invalidCharPattern.ReplaceAllString(whitespaceReplacedWithDash, "")
	removedPrefixSuffixDashses := strings.Trim(removedInvalidChars, "-")

	out := removedPrefixSuffixDashses
	return out
}

func (w Workspace) GetID() string {
	return w.ID
}

func (w Workspace) GetSSHURL() string {
	return "ssh-" + w.DNS
}

func (w Workspace) createSimpleName() WorkspaceLocalID {
	return WorkspaceLocalID(CleanSubdomain(w.Name))
}

func (w Workspace) GetNodeIdentifierForVPN() string {
	return string(w.createSimpleName())
}

type OnboardingStatus struct {
	Editor  string `json:"editor"`
	SSH     bool   `json:"ssh"`
	UsedCLI bool   `json:"usedCli"`
}

// https://stackoverflow.com/questions/27545270/how-to-get-a-value-from-map/
func safeStringMap(mapStrInter map[string]interface{}, key, fallback string) string {
	var value string
	var ok bool
	if x, found := mapStrInter[key]; found {
		if value, ok = x.(string); !ok {
			// do whatever you want to handle errors - this means this wasn't a string
			return fallback
		}
		return value
	}
	return fallback
}

// https://stackoverflow.com/questions/27545270/how-to-get-a-value-from-map/
func safeBoolMap(mapStrInter map[string]interface{}, key string, fallback bool) bool {
	var value bool
	var ok bool
	if x, found := mapStrInter[key]; found {
		if value, ok = x.(bool); !ok {
			return fallback
		}
		return value
	}
	return fallback
}

func (u User) GetOnboardingStatus() (*OnboardingStatus, error) {
	x := &OnboardingStatus{
		Editor:  safeStringMap(u.OnboardingStatus, "editor", ""), // empty string is the false state here
		SSH:     safeBoolMap(u.OnboardingStatus, "SSH", false),
		UsedCLI: safeBoolMap(u.OnboardingStatus, "usedCLI", false),
	}
	return x, nil
}

type VirtualProject struct {
	Name             string
	GitURL           string
	WorkspacesByUser map[string][]Workspace
}

func NewVirtualProjects(workspaces []Workspace) []VirtualProject {
	gitRepoWorkspaceMap := make(map[string]map[string][]Workspace)
	for _, w := range workspaces {
		if _, ok := gitRepoWorkspaceMap[w.GitRepo]; !ok {
			gitRepoWorkspaceMap[w.GitRepo] = make(map[string][]Workspace)
		}
		gitRepoWorkspaceMap[w.GitRepo][w.CreatedByUserID] = append(gitRepoWorkspaceMap[w.GitRepo][w.CreatedByUserID], w)
	}
	var projects []VirtualProject
	for k, v := range gitRepoWorkspaceMap {
		key, ok := GetFirstKeyMap(v)
		if !ok {
			continue
		}

		if len(v[key]) == 0 {
			continue
		}

		projectName := v[key][0].Name // TODO this is the SUPER hacky unexpected behavior part
		projects = append(projects, VirtualProject{Name: projectName, GitURL: k, WorkspacesByUser: v})
	}
	return projects
}

func GetFirstKeyMap(strMap map[string][]Workspace) (string, bool) {
	for k := range strMap {
		return k, true
	}
	return "", false
}

func (v VirtualProject) GetUserWorkspaces(userID string) []Workspace {
	return v.WorkspacesByUser[userID]
}

func (v VirtualProject) GetUniqueUserCount() int {
	return len(v.WorkspacesByUser)
}
