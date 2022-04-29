package entity

import (
	"fmt"
	"strings"
)

type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type UpdateUser struct {
	Username          string `json:"username"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	BaseWorkspaceRepo string `json:"baseWorkspaceRepo"`
}

type User struct {
	ID                string `json:"id"`
	PublicKey         string `json:"publicKey,omitempty"`
	Username          string `json:"username"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	WorkspacePassword string `json:"workspacePassword"`
	BaseWorkspaceRepo string `json:"baseWorkspaceRepo"`
	GlobalUserType    string `json:"globalUserType"`
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

const featureSimpleNames = false

func (w Workspace) GetLocalIdentifier(workspaces []Workspace) WorkspaceLocalID {
	if featureSimpleNames {
		return w.createSimpleNameForWorkspace(workspaces)
	} else {
		dnsSplit := strings.Split(w.DNS, "-")
		return WorkspaceLocalID(strings.Join(dnsSplit[:2], "-"))
	}
}

func (w Workspace) GetID() string {
	return w.ID
}

func (w Workspace) GetSSHURL() string {
	return "ssh-" + w.DNS
}

func makeNameSafeForEmacs(name string) string {
	splitBySlash := strings.Split(name, "/")

	concatenated := strings.Join(splitBySlash, "-")

	splitByColon := strings.Split(concatenated, ":")

	emacsSafeString := strings.Join(splitByColon, "-")

	return emacsSafeString
}

func (w Workspace) createSimpleNameForWorkspace(workspaces []Workspace) WorkspaceLocalID {
	isUnique := true
	if len(workspaces) > 0 {
		for _, v := range workspaces {
			/*
				If it's a:
					- different workspace
					- for the same user
					- with the same name
				it needs entropy
			*/

			if v.ID != w.ID && v.CreatedByUserID == w.CreatedByUserID && v.Name == w.Name {
				isUnique = false
				break
			}
		}
	}

	if isUnique {
		sanitizedName := makeNameSafeForEmacs(w.Name)
		return WorkspaceLocalID(sanitizedName)
	} else {
		dnsSplit := strings.Split(w.DNS, "-")
		return WorkspaceLocalID(strings.Join(dnsSplit[:2], "-"))
	}
}

func (w Workspace) GetNodeIdentifierForVPN(workspaces []Workspace) string {
	return string(w.createSimpleNameForWorkspace(workspaces))
}

type OnboardingStatus struct {
	Editor  string `json:"editor"`
	Ssh     bool   `json:"ssh"`
	UsedCLI bool   `json:"usedCli"`
}

func (u User) GetOnboardingStatus() (*OnboardingStatus, error) {
	// TODO: get actual status
	return &OnboardingStatus{
		Editor:  "", // empty string is the false state here
		Ssh:     false,
		UsedCLI: true,
	}, nil
}

func (u User) UpdateOnboardingEditorStatus(editor string) {
	// TODO: implement me
}

func (u User) UpdateOnboardingSSHStatus(ssh bool) {
	// TODO: implement me
}

func (u User) UpdateOnboardingCLIStatus(usedCLI bool) {
	// TODO: implement me
}
