package store

import (
	"fmt"
	"testing"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestGetCurrentUser(t *testing.T) {
	expected := &entity.User{
		ID: "1",
	}
	s := MakeMockAuthHTTPStore().withDevPlaneServices(&devPlaneServices{
		user: &mockDevPlaneUserService{currentUser: &nodev1.User{UserId: expected.ID}},
	})

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
	expected := &entity.UserKeys{
		PrivateKey: "priv",
		PublicKey:  "pub",
	}
	s := MakeMockAuthHTTPStore().withDevPlaneServices(&devPlaneServices{
		user: &mockDevPlaneUserService{keys: expected},
	})

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

	expected := &UserCreateResponse{
		User: entity.User{ID: "1"},
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
	if !assert.Equal(t, &expected.User, u) {
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
	url := fmt.Sprintf("=~^%s/%s.+", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(usersIDPathPattern, expected.ID))
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
