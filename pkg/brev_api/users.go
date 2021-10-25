package brev_api

import (
	"github.com/brevdev/brev-cli/pkg/requests"
)

type User struct {
	Id string `json:"id"`
}

func (a *Client) GetMe() (*User, error) {
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: brevEndpoint("/api/me"),
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

	var payload User
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

	return &payload, nil
}

type PrivateKeys struct {
	Cert          string
	SSHPrivateKey string
}

func (a *Client) GetMePrivateKeys() (*PrivateKeys, error) {
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: brevEndpoint("/api/me/privatekeys"),
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
	var payload PrivateKeys
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

	return &payload, nil
}
