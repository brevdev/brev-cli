package brev_api

import (
	"github.com/brevdev/brev-cli/pkg/requests"
)

type RequestCreateWorkspace struct {
	Name       string   `json:"name"`
    WorkspaceGroupId string `json:"workspaceGroupId"`
    WorkspaceClassId string `json:"workspaceClassId"`
    GitRepo string `json:"gitRepo"`;
	WorkspaceTemplateId string `json:"workspaceTemplateId"`
}
type Workspace struct {
	Id         string   `json:"id"`
	Name       string   `json:"name"`
    WorkspaceGroupId string `json:"workspaceGroupId"`
    OrganizationId string `json:"organizationId"`
    WorkspaceClassId string `json:"workspaceClassId"`
    CreatedByUserId string `json:"createdByUserId"`
    Dns string `json:"dns"`
	Status string `json:"status"`
    Password string `json:"password"`
    GitRepo string `json:"gitRepo"`;
}

func (a *Client) GetWorkspaces(orgID string) ([]Workspace, error) {

	// t := terminal.New()

	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: brevEndpoint("/api/organizations/" + orgID + "/workspaces"),
		QueryParams: []requests.QueryParam{
			{"utm_source", "cli"},
		},
		Headers: []requests.Header{
			{"Authorization", "Bearer " + a.Key.AccessToken},
		},
	}

	// t.Vprint(t.Green(brevEndpoint("/api/organizations/" + orgID + "/workspaces")))

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
			{"utm_source", "cli"},
		},
		Headers: []requests.Header{
			{"Authorization", "Bearer " + a.Key.AccessToken},
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
			{"utm_source", "cli"},
		},
		Headers: []requests.Header{
			{"Authorization", "Bearer " + a.Key.AccessToken},
		},
		Payload: RequestCreateWorkspace{
			Name: name,
			WorkspaceGroupId: "k8s.brevstack.com",
			WorkspaceClassId: "2x8",
			GitRepo: gitrepo,
			WorkspaceTemplateId: "4nbb4lg2s", // default ubuntu template
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
