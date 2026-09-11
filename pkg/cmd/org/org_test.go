package org

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOrgSetStore struct {
	authTokens          *entity.AuthTokens
	workspaceID         string
	orgs                []entity.Organization
	getOrganizations    int
	setDefaultOrgCalls  int
	defaultOrganization *entity.Organization
}

func (m *mockOrgSetStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return m.authTokens, nil
}

func (m *mockOrgSetStore) GetWorkspaces(_ string, _ *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	return nil, nil
}

func (m *mockOrgSetStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return nil, nil
}

func (m *mockOrgSetStore) GetCurrentUser() (*entity.User, error) {
	return nil, nil
}

func (m *mockOrgSetStore) GetUsers(_ map[string]string) ([]entity.User, error) {
	return nil, nil
}

func (m *mockOrgSetStore) GetWorkspace(_ string) (*entity.Workspace, error) {
	return nil, nil
}

func (m *mockOrgSetStore) GetOrganizations(_ *store.GetOrganizationsOptions) ([]entity.Organization, error) {
	m.getOrganizations++
	return m.orgs, nil
}

func (m *mockOrgSetStore) SetDefaultOrganization(org *entity.Organization) error {
	m.setDefaultOrgCalls++
	m.defaultOrganization = org
	return nil
}

func (m *mockOrgSetStore) GetServerSockFile() string {
	return ""
}

func (m *mockOrgSetStore) CreateInviteLink(_ string) (string, error) {
	return "", nil
}

func (m *mockOrgSetStore) GetCurrentWorkspaceID() (string, error) {
	return m.workspaceID, nil
}

func (m *mockOrgSetStore) CreateOrganization(_ store.CreateOrganizationRequest) (*entity.Organization, error) {
	return nil, nil
}

func TestOrgSetSwitchesOrg(t *testing.T) {
	s := &mockOrgSetStore{orgs: []entity.Organization{{ID: "org-dm", Name: "dm"}}}

	err := set("dm", s, terminal.New())
	require.NoError(t, err)
	assert.Equal(t, 1, s.getOrganizations)
	assert.Equal(t, 1, s.setDefaultOrgCalls)
	assert.Equal(t, "org-dm", s.defaultOrganization.ID)
}

func TestOrgSetNoOrgsFound(t *testing.T) {
	s := &mockOrgSetStore{} // name filter yields no orgs

	err := set("dm", s, terminal.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no orgs exist with name dm")
	assert.Equal(t, 0, s.setDefaultOrgCalls)
}
