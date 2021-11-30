package brevapi

import (
	"fmt"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/requests"
)

type User struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey,omitempty"`
	Username  string `json:"username"`
}

func (a *DeprecatedClient) GetMe() (*User, error) {
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: buildBrevEndpoint("/api/me"),
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

	var payload User
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &payload, nil
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

func (a *DeprecatedClient) GetMeKeys() (*UserKeys, error) {
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: buildBrevEndpoint("/api/me/keys"),
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
	var payload UserKeys
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &payload, nil
}

func (a *DeprecatedClient) CreateUser(identity string) (*User, error) {
	request := requests.RESTRequest{
		Method:   "POST",
		Endpoint: buildBrevEndpoint("/api/users"),
		QueryParams: []requests.QueryParam{
			{Key: "utm_source", Value: "cli"},
		},
		Headers: []requests.Header{
			{Key: "Authorization", Value: "Bearer " + a.Key.AccessToken},
			{Key: "Identity", Value: identity},
		},
	}
	response, err := request.SubmitStrict()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var payload User
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &payload, nil
}
