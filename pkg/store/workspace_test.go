package store

import (
	"fmt"
	"testing"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestGetWorkspaces(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	orgID := "o1"
	expected := []brevapi.Workspace{{
		ID:               "1",
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   orgID,
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cbuid",
		DNS:              "dns",
		Status:           "s",
		Password:         "pw",
		GitRepo:          "g",
	}}
	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceOrgPathPattern, orgID))
	httpmock.RegisterResponder("GET", url, res)

	u, err := s.GetWorkspaces(orgID, nil)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}

	if !assert.Equal(t, expected, u) {
		return
	}

	u, err = s.GetWorkspaces(orgID, &GetWorkspacesOptions{
		UserID: "",
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

func TestGetWorkspacesWithName(t *testing.T) { //nolint:dupl // To refactor later, not fully duplicate code
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	orgID := "o1"
	expected := []brevapi.Workspace{{
		ID:               "1",
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   orgID,
		WorkspaceClassID: "wci",
		CreatedByUserID:  "blas",
		DNS:              "dns",
		Status:           "s",
		Password:         "pw",
		GitRepo:          "g",
	}}

	body := append([]brevapi.Workspace{
		{
			ID:               "2",
			Name:             "n2",
			WorkspaceGroupID: "wgi",
			OrganizationID:   orgID,
			WorkspaceClassID: "wci",
			CreatedByUserID:  "other",
			DNS:              "dns",
			Status:           "s",
			Password:         "p",
			GitRepo:          "g",
		},
	}, expected...)
	res, err := httpmock.NewJsonResponder(200, body)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceOrgPathPattern, orgID))
	httpmock.RegisterResponder("GET", url, res)

	w, err := s.GetWorkspaces(orgID, &GetWorkspacesOptions{
		Name: "name",
	})
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, w) {
		return
	}
	if !assert.Len(t, w, 1) {
		return
	}
	if !assert.Equal(t, expected, w) {
		return
	}
}

func TestGetWorkspacesWithUser(t *testing.T) { //nolint:dupl // To refactor later, not fully duplicate code
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	orgID := "o1"
	expected := []brevapi.Workspace{{
		ID:               "1",
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   orgID,
		WorkspaceClassID: "wci",
		CreatedByUserID:  "me",
		DNS:              "dns",
		Status:           "s",
		Password:         "pw",
		GitRepo:          "g",
	}}

	body := append([]brevapi.Workspace{
		{
			ID:               "2",
			Name:             "n2",
			WorkspaceGroupID: "wgi",
			OrganizationID:   orgID,
			WorkspaceClassID: "wci",
			CreatedByUserID:  "other",
			DNS:              "dns",
			Status:           "s",
			Password:         "p",
			GitRepo:          "g",
		},
	}, expected...)
	res, err := httpmock.NewJsonResponder(200, body)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceOrgPathPattern, orgID))
	httpmock.RegisterResponder("GET", url, res)

	u, err := s.GetWorkspaces(orgID, &GetWorkspacesOptions{
		UserID: "me",
	})
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}
	if !assert.Len(t, u, 1) {
		return
	}
	if !assert.Equal(t, expected, u) {
		return
	}
}

func TestGetWorkspaceMetaData(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	workspaceID := "1"
	expected := &brevapi.WorkspaceMetaData{
		PodName:       "pn",
		NamespaceName: "nsn",
	}

	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceMetadataPathPattern, workspaceID))
	httpmock.RegisterResponder("GET", url, res)

	u, err := s.GetWorkspaceMetaData(workspaceID)
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

func TestStopWorkspace(t *testing.T) { //nolint:dupl // ok to have this be duplicate
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	workspaceID := "1"
	expected := &brevapi.Workspace{
		ID:               workspaceID,
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "dns",
		Status:           "s",
		Password:         "p",
		GitRepo:          "g",
	}

	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceStopPathPattern, workspaceID))
	httpmock.RegisterResponder("PUT", url, res)

	u, err := s.StopWorkspace(workspaceID)
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

func TestStartWorkspace(t *testing.T) { //nolint:dupl // ok to have this be duplicate
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	workspaceID := "1"
	expected := &brevapi.Workspace{
		ID:               workspaceID,
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "dns",
		Status:           "s",
		Password:         "p",
		GitRepo:          "g",
	}

	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceStartPathPattern, workspaceID))
	httpmock.RegisterResponder("PUT", url, res)

	u, err := s.StartWorkspace(workspaceID)
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

func TestGetWorkspace(t *testing.T) { //nolint:dupl // ok to have this be duplicate
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	workspaceID := "1"
	expected := &brevapi.Workspace{
		ID:               workspaceID,
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "dns",
		Status:           "s",
		Password:         "p",
		GitRepo:          "g",
	}

	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspacePathPattern, workspaceID))
	httpmock.RegisterResponder("GET", url, res)

	u, err := s.GetWorkspace(workspaceID)
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
