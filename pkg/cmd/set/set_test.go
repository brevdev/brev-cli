package set

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSetStore struct {
	authTokens          *entity.AuthTokens
	workspaceID         string
	orgs                []entity.Organization
	getOrganizations    int
	setDefaultOrgCalls  int
	defaultOrganization *entity.Organization
}

func (m *mockSetStore) GetWorkspaces(_ string, _ *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	return nil, nil
}

func (m *mockSetStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return nil, nil
}

func (m *mockSetStore) GetCurrentUser() (*entity.User, error) {
	return nil, nil
}

func (m *mockSetStore) SetDefaultOrganization(org *entity.Organization) error {
	m.setDefaultOrgCalls++
	m.defaultOrganization = org
	return nil
}

func (m *mockSetStore) GetOrganizations(_ *store.GetOrganizationsOptions) ([]entity.Organization, error) {
	m.getOrganizations++
	return m.orgs, nil
}

func (m *mockSetStore) GetServerSockFile() string {
	return ""
}

func (m *mockSetStore) GetCurrentWorkspaceID() (string, error) {
	return m.workspaceID, nil
}

func (m *mockSetStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return m.authTokens, nil
}

func TestSetSwitchesOrg(t *testing.T) {
	s := &mockSetStore{orgs: []entity.Organization{{ID: "org-dm", Name: "dm"}}}

	err := set("dm", s)
	require.NoError(t, err)
	assert.Equal(t, 1, s.getOrganizations)
	assert.Equal(t, 1, s.setDefaultOrgCalls)
	assert.Equal(t, "org-dm", s.defaultOrganization.ID)
}

func TestSetNoOrgsFound(t *testing.T) {
	s := &mockSetStore{} // name filter yields no orgs

	err := set("dm", s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no orgs exist with name dm")
	assert.Equal(t, 1, s.getOrganizations)
	assert.Equal(t, 0, s.setDefaultOrgCalls)
}

func TestSetRejectsInsideWorkspace(t *testing.T) {
	s := &mockSetStore{workspaceID: "ws-1"}

	err := set("dm", s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can not set orgs in a workspace")
	assert.Equal(t, 0, s.getOrganizations)
}
