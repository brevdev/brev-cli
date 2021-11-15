package brevapi

import (
	"github.com/brevdev/brev-cli/pkg/brev_errors"
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
		return nil, brev_errors.WrapAndTrace(err)
	}
	var payload []Organization
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, brev_errors.WrapAndTrace(err)
	}

	err = WriteOrgCache(payload)
	if err != nil {
		// do nothing, failure to cache shouldn't kill this command
	}

	return payload, nil
}

func GetActiveOrgContext(fs afero.Fs) (*Organization, error) {
	brevActiveOrgsFile := files.GetActiveOrgsPath()
	exists, err := afero.Exists(fs, brevActiveOrgsFile)
	if err != nil {
		return nil, brev_errors.WrapAndTrace(err)
	}

	if !exists {
		client, err := NewClient()
		if err != nil {
			return nil, err
		}
		orgs, err := client.GetOrgs()
		if err != nil {
			return nil, err
		}
		if len(orgs) > 0 {
			org := orgs[0]
			path := files.GetActiveOrgsPath()
			err := files.OverwriteJSON(path, org)
			if err != nil {
				return nil, err
			}

		}
	}

	var activeOrg Organization
	err = files.ReadJSON(brevActiveOrgsFile, &activeOrg)
	if err != nil {
		return nil, brev_errors.WrapAndTrace(err)
	}

	return &activeOrg, nil
}
