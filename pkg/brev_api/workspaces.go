package brev_api

import (
	"fmt"
	"sync"

	"github.com/brevdev/brev-cli/pkg/config"
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

// Note: this is the "projects" view
func (a *Client) GetMyWorkspaces(orgID string) ([]Workspace, error) {

	var wg sync.WaitGroup

	wg.Add(1)
	me := make(chan *User)
	err := make(chan error)
	go func() {
		fmt.Println("fetching me.")
		me_, err_ := a.GetMe()
		me <- me_
		err <- err_
		defer wg.Done()
	}()

	wg.Add(1)

	payload := make(chan []Workspace)
	err2 := make(chan error)
	go func() {
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
		
		fmt.Println("fetching " + buildBrevEndpoint("/api/organizations/" + orgID + "/workspaces"))
		response, err := request.SubmitStrict()
		if err != nil {
			err2 <- err
			defer wg.Done()
			// 
			// return nil, err
		}
	
		var payload_ []Workspace
		err = response.UnmarshalPayload(&payload_)
		if err != nil {
			err2 <- err
			defer wg.Done()
			// return nil, err
		}
		payload <- payload_
		defer wg.Done()
	}()
	wg.Wait()

	err_1 := <- err
	err_2 := <- err2
	if err_1 != nil {
		return nil, err_1
	} else if err_2 != nil {
		return nil, err_2
	}


	payload_ := <- payload
	me_ := <- me
	var myWorkspaces []Workspace
	for _, w := range payload_ {
		if w.CreatedByUserID == me_.Id {
			myWorkspaces = append(myWorkspaces, w)
		}
	}

	return myWorkspaces, nil
}

func (a *Client) GetWorkspaces(orgID string) ([]Workspace, error) {
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
		return nil, err
	}

	var payload []Workspace
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

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

func (a *Client) GetWorkspaceMetaData(wsID string) (*WorkspaceMetaData, error) {
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: buildBrevEndpoint("/api/workspaces/" + wsID + "/metadata"),
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

	var payload WorkspaceMetaData
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

	return &payload, nil
}

type AllWorkspaceData struct {
	WorkspaceMetaData
	Workspace
}

func (a *Client) GetWorkspace(wsID string) (*Workspace, error) {
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
			Name:                name,
			WorkspaceGroupID:    clusterID,
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
