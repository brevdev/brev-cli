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
	workspaceID           string
	orgs                  []entity.Organization
	getOrganizations      int
	setDefaultOrgCalls    int
	activateUserCredCalls int
	defaultOrganization   *entity.Organization
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

func (m *mockOrgSetStore) GetCurrentWorkspaceID() (string, error) {
	return m.workspaceID, nil
}

func (m *mockOrgSetStore) ActivateUserCredential() error {
	m.activateUserCredCalls++
	return nil
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
