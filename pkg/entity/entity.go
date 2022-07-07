package entity

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type IDEConfig struct {
	DefaultWorkingDir string       `json:"defaultWorkingDir"`
	VSCode            VSCodeConfig `json:"vscode"`
} // @Name IDEConfig

type VSCodeConfig struct {
	Extensions []VscodeExtensionMetadata `json:"extensions"`
} // @Name VSCodeConfig

type VscodeExtensionMetadata struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Version     string `json:"version"`
	Publisher   string `json:"publisher"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
} // @Name ExtensionMetadata

type (
	RepoName string
	RepoV0   struct {
		Repository    string   `json:"repository"`
		Branch        string   `json:"branch"` // branch, tag, commit
		Directory     string   `json:"directory"`
		BrevPath      string   `json:"brevPath"`
		SetupExecPath string   `json:"setupExecPath"`
		ExecWorkDir   string   `json:"execWorkDir"`
		DependsOn     []string `json:"dependsOn"`
	}
	ReposV0 map[RepoName]RepoV0
	RepoV1  struct {
		Type RepoType `json:"type"`
		GitRepo
		EmptyRepo
	}
	ReposV1 map[RepoName]RepoV1
)
type RepoType string

func (r RepoV1) GetDir() (string, error) {
	if r.Type == GitRepoType { //nolint:gocritic // i like ifs
		return r.GitRepo.GetDir(), nil
	} else if r.Type == EmptyRepoType {
		return r.EmptyDirectory, nil
	} else {
		return "", fmt.Errorf("error: invalid type in getting dir of repov1")
	}
}

const (
	GitRepoType   RepoType = "git"
	EmptyRepoType RepoType = "empty"
)

type GitRepo struct {
	Repository string `json:"repository,omitempty"`
	GitRepoOptions
}

func (g GitRepo) GetDir() string {
	if g.GitDirectory != nil && *g.GitDirectory != "" {
		return *g.GitDirectory
	} else {
		repo := g.Repository
		return strings.Split(repo[strings.LastIndex(repo, "/")+1:], ".")[0]
	}
}

type GitRepoOptions struct {
	Branch       *string `json:"branch,omitempty"`           // branch, tag, commit
	GitDirectory *string `json:"gitRepoDirectory,omitempty"` // need to be different names than emptyrepo
}
type EmptyRepo struct {
	EmptyDirectory string `json:"emptyRepoDirectory,omitempty"` // need to be different names than gitrepo
}

type (
	ExecName string
	ExecV0   struct {
		Exec        string   `json:"exec"`
		ExecWorkDir string   `json:"execWorkDir"`
		DependsOn   []string `json:"dependsOn"`
	}
	ExecsV0 map[ExecName]ExecV0
	ExecV1  struct {
		Type  ExecType   `json:"type"`  // string or path // default=str
		Stage *ExecStage `json:"stage"` // start, build // default=start
		ExecOptions
		StringExec
		PathExec
	}
	ExecOptions struct {
		IsDisabled     bool       `json:"isDisabled"`
		ExecWorkDir    *string    `json:"execWorkDir"`
		LogPath        *string    `json:"logPath"`
		LogArchivePath *string    `json:"logArchivePath"`
		DependsOn      []ExecName `json:"dependsOn"`
	}
	ExecsV1 map[ExecName]ExecV1
)

type ExecType string

const StringExecType ExecType = "string"

type StringExec struct {
	ExecStr string `json:"execStr,omitempty"`
}

const PathExecType ExecType = "path"

type PathExec struct {
	ExecPath string `json:"execPath,omitempty"`
}

type ExecStage string

const (
	StartStage ExecStage = "start"
	BuildStage ExecStage = "build"
)

type UpdateUser struct {
	Username          string                 `json:"username,omitempty"`
	Name              string                 `json:"name,omitempty"`
	Email             string                 `json:"email,omitempty"`
	BaseWorkspaceRepo string                 `json:"baseWorkspaceRepo,omitempty"`
	OnboardingStatus  map[string]interface{} `json:"onboardingData,omitempty"` // todo fix inconsitency
	IdeConfig         IDEConfig              `json:"ideConfig,omitempty"`
}

type GlobalUserType string

const (
	Admin    GlobalUserType = "Admin"
	Standard GlobalUserType = "Standard"
)

type User struct {
	ID                string                 `json:"id"`
	PublicKey         string                 `json:"publicKey,omitempty"`
	Username          string                 `json:"username"`
	Name              string                 `json:"name"`
	Email             string                 `json:"email"`
	WorkspacePassword string                 `json:"workspacePassword"`
	BaseWorkspaceRepo string                 `json:"baseWorkspaceRepo"`
	GlobalUserType    GlobalUserType         `json:"globalUserType"`
	IdeConfig         IDEConfig              `json:"ideConfig,omitempty"`
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

func (v VscodeExtensionMetadata) GetID() string {
	return fmt.Sprintf("%s.%s", v.Publisher, v.Name)
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

// Workspace Status
const (
	Running   = "RUNNING"
	Starting  = "STARTING"
	Stopping  = "STOPPING"
	Deploying = "DEPLOYING"
	Stopped   = "STOPPED"
	Deleting  = "DELETING"
	Failure   = "FAILURE"
)

// Health Status
const (
	Healthy     = "HEALTHY"
	Unhealthy   = "UNHEALTHY"
	Unavailable = "UNAVAILABLE"
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

	StartupScriptPath string    `json:"startupScriptPath"`
	Repos             ReposV0   `json:"repos"`
	Execs             ExecsV0   `json:"execs"`
	IDEConfig         IDEConfig `json:"ideConfig"`

	// PrimaryApplicationId         string `json:"primaryApplicationId,omitempty"`
	// LastOnlineAt         string `json:"lastOnlineAt,omitempty"`
	// CreatedAt         string `json:"createdAt,omitempty"`
	// UpdatedAt         string `json:"updatedAt,omitempty"`
	HealthStatus  string        `json:"healthStatus"`
	IsStoppable   bool          `json:"isStoppable"`
	StatusMessage string        `json:"statusMessage"`
	StopTimeout   time.Duration `json:"stopTimeout"`
}

func (w Workspace) GetStopTimeout() time.Duration {
	return w.StopTimeout
}

func (w Workspace) GetIsStoppable() bool {
	return w.IsStoppable
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
	if w.IDEConfig.DefaultWorkingDir != "" { //nolint:gocritic // i like if else
		if path.IsAbs(w.IDEConfig.DefaultWorkingDir) {
			return w.IDEConfig.DefaultWorkingDir
		} else {
			folderName = w.IDEConfig.DefaultWorkingDir
		}
	} else if w.GitRepo != "" {
		folderName = GetDefaultProjectFolderNameFromRepo(w.GitRepo)
	} else {
		folderName = w.Name
	}

	return filepath.Join("/home/brev/workspace/", folderName) // TODO make workspace dir configurable
}

func GetDefaultProjectFolderNameFromRepo(repo string) string {
	return strings.Split(repo[strings.LastIndex(repo, "/")+1:], ".")[0]
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
