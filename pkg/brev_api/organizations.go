package brev_api

import "github.com/brevdev/brev-cli/pkg/requests"


type Organization struct {
	Id         string   `json:"id"`
	Name       string   `json:"name"`
}

type Organizations struct {
	organizations []Organization `json:"organizations"`
}


func (a *Agent) GetOrgs() ([]Organization, error) {
	
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: brevEndpoint("/api/organizations"),
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
	
	var payload []Organization
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}