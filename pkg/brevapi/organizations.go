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

func (a *DeprecatedClient) GetOrgs() ([]Organization, error) {
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
		client, err2 := NewDeprecatedClient()
		if err2 != nil {
			return nil, breverrors.WrapAndTrace(err2)
		}
		orgs, err2 := client.GetOrgs()
		if err2 != nil {
			return nil, breverrors.WrapAndTrace(err2)
		}
		if len(orgs) > 0 {
			org := orgs[0]
			path := files.GetActiveOrgsPath()
			err3 := files.OverwriteJSON(path, org)
			if err3 != nil {
				return nil, breverrors.WrapAndTrace(err3)
			}
		}
	}

	var activeOrg Organization
	err = files.ReadJSON(fs, brevActiveOrgsFile, &activeOrg)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &activeOrg, nil
}
