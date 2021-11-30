package brevapi

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/requests"
)

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

type Workspace struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	WorkspaceGroupID string `json:"workspaceGroupId"`
	OrganizationID   string `json:"organizationId"`
	WorkspaceClassID string `json:"workspaceClassId"`
	CreatedByUserID  string `json:"createdByUserId"`
	DNS              string `json:"dns"`
	Status           string `json:"status"`
	Password         string `json:"password"`
	GitRepo          string `json:"gitRepo"`
}

func (w Workspace) GetID() string {
	return w.ID
}

var (
	DefaultApplicationID = "92f59a4yf"
	DefaultApplication   = Application{
		ID:           "92f59a4yf",
		Name:         "VSCode",
		Port:         22778,
		StartCommand: "",
		Version:      "1.57.1",
	}
)
var DefaultApplicationList = []Application{DefaultApplication}

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

func (a *DeprecatedClient) GetMyWorkspaces(orgID string) ([]Workspace, error) {
	me, err := a.GetMe()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: buildBrevEndpoint("/api/organizations/" + orgID + "/workspaces"),
		QueryParams: []requests.QueryParam{
			{Key: "utm_source", Value: "cli"},
		},
		Headers: []requests.Header{
			{Key: "Authorization", Value: "Bearer " + a.Key.AccessToken},
		},
	}

	response, err := request.SubmitStrict()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var payload []Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var myWorkspaces []Workspace
	for _, w := range payload {
		if w.CreatedByUserID == me.ID {
			myWorkspaces = append(myWorkspaces, w)
		}
	}

	return myWorkspaces, nil
}
