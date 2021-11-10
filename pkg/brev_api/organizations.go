package brev_api

import (
	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/requests"
)

type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (a *Client) GetOrgs() ([]Organization, error) {
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: buildBrevEndpoint("/api/organizations"),
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

	var payload []Organization
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func GetActiveOrgContext() (*Organization, error) {
	brevActiveOrgsFile := files.GetActiveOrgsPath()
	exists, err := files.Exists(brevActiveOrgsFile, false)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, &brev_errors.ActiveOrgFileNotFound{}
	}

	var activeOrg Organization
	err = files.ReadJSON(brevActiveOrgsFile, &activeOrg)
	if err != nil {
		return nil, err
	}

	return &activeOrg, nil
}
