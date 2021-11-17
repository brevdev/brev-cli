package store

import (
	"fmt"
	"testing"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestGetCurrentUser(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	expected := &brevapi.User{
		ID: "1",
	}
	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, userPath)
	httpmock.RegisterResponder("GET", url, res)

	u, err := s.GetCurrentUser()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}

	if !assert.Equal(t, expected, u) {
		return
	}
}

func TestGetCurrentUserKeys(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	expected := &brevapi.UserKeys{
		PrivateKey: "priv",
		PublicKey:  "pub",
		WorkspaceGroups: []brevapi.WorkspaceGroupKeys{
			{
				GroupID: "gi",
				Cert:    "cert",
				CA:      "ca",
				APIURL:  "url",
			},
		},
	}
	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, userKeysPath)
	httpmock.RegisterResponder("GET", url, res)

	u, err := s.GetCurrentUserKeys()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}

	if !assert.Equal(t, expected, u) {
		return
	}
}
