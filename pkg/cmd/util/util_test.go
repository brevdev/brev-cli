package util

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWorkspaceLookupStore struct {
	t          *testing.T
	tokens     *entity.AuthTokens
	org        *entity.Organization
	workspaces []entity.Workspace
}

func (m *mockWorkspaceLookupStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return m.tokens, nil
}

func (m *mockWorkspaceLookupStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return m.org, nil
}

func (m *mockWorkspaceLookupStore) GetWorkspaceByNameOrID(_ string, _ string) ([]entity.Workspace, error) {
	return m.workspaces, nil
}

func (m *mockWorkspaceLookupStore) GetCurrentUser() (*entity.User, error) {
	m.t.Fatal("api-key workspace lookup must not fetch current user")
	return nil, nil
}

func TestGetUserWorkspaceByNameOrIDErr_APIKeyRejectsAmbiguousMatches(t *testing.T) {
	store := &mockWorkspaceLookupStore{
		t:      t,
		tokens: &entity.AuthTokens{APIKey: auth.BrevAPIKeyPrefix + "test-key"},
		org:    &entity.Organization{ID: "org-test"},
		workspaces: []entity.Workspace{
			{ID: "workspace-1", Name: "dev", Status: entity.Running},
			{ID: "workspace-2", Name: "dev", Status: entity.Stopped},
		},
	}

	workspace, err := GetUserWorkspaceByNameOrIDErr(store, "dev")

	require.Error(t, err)
	assert.Nil(t, workspace)
	assert.Contains(t, err.Error(), "multiple instances found with id/name dev")
}

func TestGetUserWorkspaceByNameOrIDErr_APIKeyUsesOnlyNonFailedMatch(t *testing.T) {
	store := &mockWorkspaceLookupStore{
		t:      t,
		tokens: &entity.AuthTokens{APIKey: auth.BrevAPIKeyPrefix + "test-key"},
		org:    &entity.Organization{ID: "org-test"},
		workspaces: []entity.Workspace{
			{ID: "failed-workspace", Name: "dev", Status: entity.Failure},
			{ID: "running-workspace", Name: "dev", Status: entity.Running},
		},
	}

	workspace, err := GetUserWorkspaceByNameOrIDErr(store, "dev")

	require.NoError(t, err)
	require.NotNil(t, workspace)
	assert.Equal(t, "running-workspace", workspace.ID)
}
