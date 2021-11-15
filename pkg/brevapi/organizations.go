package brevapi

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/requests"
	"github.com/spf13/afero"
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
		return nil, breverrors.WrapAndTrace(err)
	}
	var payload []Organization
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	_ = WriteOrgCache(payload)

	return payload, nil
}

func GetActiveOrgContext(fs afero.Fs) (*Organization, error) {
	brevActiveOrgsFile := files.GetActiveOrgsPath()
	exists, err := afero.Exists(fs, brevActiveOrgsFile)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if !exists {
		client, err := NewClient()
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		orgs, err := client.GetOrgs()
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if len(orgs) > 0 {
			org := orgs[0]
			path := files.GetActiveOrgsPath()
			err := files.OverwriteJSON(path, org)
			if err != nil {
				return nil, breverrors.WrapAndTrace(err)
			}
		}
	}

	var activeOrg Organization
	err = files.ReadJSON(brevActiveOrgsFile, &activeOrg)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &activeOrg, nil
}
