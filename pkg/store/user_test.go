package store

import (
	"fmt"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestGetCurrentUser(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	expected := &entity.User{
		ID: "1",
	}
	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, mePath)
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

	expected := &entity.UserKeys{
		PrivateKey: "priv",
		PublicKey:  "pub",
		WorkspaceGroups: []entity.WorkspaceGroupKeys{
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

func TestCreateUser(t *testing.T) {
	s := MakeMockNoHTTPStore()
	httpmock.ActivateNonDefault(s.noAuthHTTPClient.restyClient.GetClient())

	expected := &entity.User{
		ID: "1",
	}
	res, err := httpmock.NewJsonResponder(201, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.noAuthHTTPClient.restyClient.BaseURL, usersPath)
	httpmock.RegisterResponder("POST", url, res)

	u, err := s.CreateUser("identityToken")
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

func TestUpdateUser(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	expected := &entity.User{
		ID: "testUserId",
	}
	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s?utm_source=cli", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(usersIDPathPattern, expected.ID))
	httpmock.RegisterResponder("PUT", url, res)

	u, err := s.UpdateUser(expected.ID, &entity.UpdateUser{
		Username:          "",
		Name:              "",
		Email:             "",
		BaseWorkspaceRepo: "",
	})
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

func TestApproveUser(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	expected := &entity.User{
		ID: "1",
	}
	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s/%s/approve?utm_source=cli", s.authHTTPClient.restyClient.BaseURL, usersPath, expected.ID)
	httpmock.RegisterResponder("POST", url, res)

	u, err := s.ApproveUserByID(expected.ID)
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
