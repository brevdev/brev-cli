package brev_api

import (
	"github.com/brevdev/brev-cli/pkg/requests"
)

type RequestCreateWorkspace struct {
	Name                string `json:"name"`
	WorkspaceGroupID    string `json:"workspaceGroupId"`
	WorkspaceClassID    string `json:"workspaceClassId"`
	GitRepo             string `json:"gitRepo"`
	WorkspaceTemplateID string `json:"workspaceTemplateId"`
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

func (a *Client) GetWorkspaces(orgID string) ([]Workspace, error) {

	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: brevEndpoint("/api/organizations/" + orgID + "/workspaces"),
		QueryParams: []requests.QueryParam{
			{Key: "utm_source", Value: "cli"},
		},
		Headers: []requests.Header{
			{Key: "Authorization", Value: "Bearer " + a.Key.AccessToken},
		},
	}

	response, err := request.SubmitStrict()
	if err != nil {
		return nil, err
	}

	var payload []Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (a *Client) GetWorkspace(wsID string) (*Workspace, error) {

	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: brevEndpoint("/api/workspaces/" + wsID),
		QueryParams: []requests.QueryParam{
			{Key: "utm_source", Value: "cli"},
		},
		Headers: []requests.Header{
			{Key: "Authorization", Value: "Bearer " + a.Key.AccessToken},
		},
	}

	response, err := request.SubmitStrict()
	if err != nil {
		return nil, err
	}

	var payload Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

	return &payload, nil
}

func (a *Client) CreateWorkspace(orgID string, name string, gitrepo string) (*Workspace, error) {

	request := &requests.RESTRequest{
		Method:   "POST",
		Endpoint: brevEndpoint("/api/organizations/" + orgID + "/workspaces"),
		QueryParams: []requests.QueryParam{
			{Key: "utm_source", Value: "cli"},
		},
		Headers: []requests.Header{
			{Key: "Authorization", Value: "Bearer " + a.Key.AccessToken},
		},
		Payload: RequestCreateWorkspace{
			Name:                name,
			WorkspaceGroupID:    "k8s.brevstack.com",
			WorkspaceClassID:    "2x8",
			GitRepo:             gitrepo,
			WorkspaceTemplateID: "4nbb4lg2s", // default ubuntu template
		},
	}

	response, err := request.SubmitStrict()
	if err != nil {
		return nil, err
	}

	var payload Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

	return &payload, nil
}
