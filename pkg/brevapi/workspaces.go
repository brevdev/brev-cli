package brevapi

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/config"
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

// Note: this is the "projects" view
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

func (a *DeprecatedClient) GetWorkspaces(orgID string) ([]Workspace, error) {
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

	_ = WriteIndividualWorkspaceCache(orgID, payload)

	return payload, nil
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

func (a *DeprecatedClient) GetWorkspaceMetaData(wsID string) (*WorkspaceMetaData, error) {
	ep := buildBrevEndpoint("/api/workspaces/" + wsID + "/metadata")
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: ep,
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

	var payload WorkspaceMetaData
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &payload, nil
}

type WorkspaceWithMeta struct {
	WorkspaceMetaData
	Workspace
}

func (a *DeprecatedClient) GetWorkspace(wsID string) (*Workspace, error) {
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: buildBrevEndpoint("/api/workspaces/" + wsID),
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

	var payload Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &payload, nil
}

func (a *DeprecatedClient) CreateWorkspace(orgID string, name string, gitrepo string) (*Workspace, error) {
	clusterID := config.GlobalConfig.GetDefaultClusterID()

	request := &requests.RESTRequest{
		Method:   "POST",
		Endpoint: buildBrevEndpoint("/api/organizations/" + orgID + "/workspaces"),
		QueryParams: []requests.QueryParam{
			{Key: "utm_source", Value: "cli"},
		},
		Headers: []requests.Header{
			{Key: "Authorization", Value: "Bearer " + a.Key.AccessToken},
		},
		Payload: RequestCreateWorkspace{
			Name:                 name,
			WorkspaceGroupID:     clusterID,
			WorkspaceClassID:     "2x8",
			GitRepo:              gitrepo,
			WorkspaceTemplateID:  "4nbb4lg2s", // default ubuntu template
			PrimaryApplicationID: DefaultApplicationID,
			Applications:         []Application{DefaultApplication},
		},
	}

	response, err := request.SubmitStrict()
	if err != nil {
		fmt.Println(err.Error())
		return nil, breverrors.WrapAndTrace(err)
	}

	var payload Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &payload, nil
}

func (a *DeprecatedClient) DeleteWorkspace(wsID string) (*Workspace, error) {
	request := requests.RESTRequest{
		Method:   "DELETE",
		Endpoint: buildBrevEndpoint("/api/workspaces/" + wsID),
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

	var payload Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &payload, nil
}

func (a *DeprecatedClient) StartWorkspace(wsID string) (*Workspace, error) {
	request := requests.RESTRequest{
		Method:   "PUT",
		Endpoint: buildBrevEndpoint("/api/workspaces/" + wsID + "/start"),
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

	var payload Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &payload, nil
}

func (a *DeprecatedClient) StopWorkspace(wsID string) (*Workspace, error) {
	request := requests.RESTRequest{
		Method:   "PUT",
		Endpoint: buildBrevEndpoint("/api/workspaces/" + wsID + "/stop"),
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

	var payload Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &payload, nil
}

func (a *DeprecatedClient) ResetWorkspace(wsID string) (*Workspace, error) {
	request := requests.RESTRequest{
		Method:   "PUT",
		Endpoint: buildBrevEndpoint("/api/workspaces/" + wsID + "/reset"),
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

	var payload Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &payload, nil
}
