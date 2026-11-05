package delete

import (
	"context"
	"errors"
	"testing"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOrgID      = "org-test"
	testUserID     = "user-me"
	testTeammateID = "user-teammate"
)

type mockDeleteStore struct {
	user           *entity.User
	tokens         *entity.AuthTokens
	workspaces     []entity.Workspace
	environments   map[string]*devplanev1.Environment
	deletedIDs     []string
	userLookupErr  error
	orgLookupErr   error
	orgLookupCalls int
}

// newMockDeleteStore gives every instance a running dev-plane environment.
func newMockDeleteStore(workspaces ...entity.Workspace) *mockDeleteStore {
	environments := map[string]*devplanev1.Environment{}
	for _, w := range workspaces {
		environments[w.ID] = &devplanev1.Environment{Status: devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING}
	}
	return &mockDeleteStore{
		user:         &entity.User{ID: testUserID, GlobalUserType: entity.Standard},
		workspaces:   workspaces,
		environments: environments,
	}
}

func (m *mockDeleteStore) GetAuthTokens() (*entity.AuthTokens, error) { return m.tokens, nil }

func (m *mockDeleteStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return &entity.Organization{ID: testOrgID}, nil
}

func (m *mockDeleteStore) GetCurrentUser() (*entity.User, error) { return m.user, nil }

func (m *mockDeleteStore) GetOrganizations(_ *store.GetOrganizationsOptions) ([]entity.Organization, error) {
	return []entity.Organization{{ID: testOrgID}}, nil
}

// GetWorkspaces applies the same name/ID and user filters as AuthHTTPStore.GetWorkspaces.
func (m *mockDeleteStore) GetWorkspaces(_ string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	if options == nil || options.UserID == "" {
		m.orgLookupCalls++
		if m.orgLookupErr != nil {
			return nil, m.orgLookupErr
		}
	}
	var matches []entity.Workspace
	for _, w := range m.workspaces {
		if options != nil && options.Name != "" && w.Name != options.Name && w.ID != options.Name {
			continue
		}
		if options != nil && options.UserID != "" && !w.CanShow(options.UserID) {
			continue
		}
		matches = append(matches, w)
	}
	return matches, nil
}

// GetWorkspaceByNameOrID keeps only the caller's own instances for logged-in users, like AuthHTTPStore.
func (m *mockDeleteStore) GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error) {
	if m.userLookupErr != nil {
		return nil, m.userLookupErr
	}
	options := &store.GetWorkspacesOptions{Name: nameOrID}
	if !auth.IsAPIKeyAuthStore(m) {
		options.UserID = m.user.ID
	}
	return m.GetWorkspaces(orgID, options)
}

func (m *mockDeleteStore) GetEnvironment(_ context.Context, environmentID string) (*devplanev1.Environment, error) {
	env, ok := m.environments[environmentID]
	if !ok {
		return nil, errors.New("environment not found")
	}
	return env, nil
}

func (m *mockDeleteStore) DeleteWorkspace(workspaceID string) (*entity.Workspace, error) {
	m.deletedIDs = append(m.deletedIDs, workspaceID)
	return &entity.Workspace{ID: workspaceID}, nil
}

func runDelete(t *testing.T, deleteStore *mockDeleteStore, workspaceName string) error {
	t.Helper()
	t.Setenv(auth.APIKeyEnvVar, "")
	return deleteWorkspace(context.Background(), workspaceName, terminal.New(), deleteStore, true)
}

func TestDeleteWorkspaceFindsTheInstance(t *testing.T) {
	workspaces := []entity.Workspace{
		{ID: "ws-mine", Name: "my-gpu", CreatedByUserID: testUserID, Status: entity.Running},
		{ID: "ws-teammate", Name: "teammate-gpu", CreatedByUserID: testTeammateID, Status: entity.Running},
		{ID: "ws-teammate-my-gpu", Name: "my-gpu", CreatedByUserID: testTeammateID, Status: entity.Running},
	}
	tests := []struct {
		name          string
		workspaceName string
		wantDeletedID string
	}{
		{name: "own instance wins over a teammate's with the same name", workspaceName: "my-gpu", wantDeletedID: "ws-mine"},
		{name: "teammate's instance by name", workspaceName: "teammate-gpu", wantDeletedID: "ws-teammate"},
		{name: "teammate's instance by ID", workspaceName: "ws-teammate", wantDeletedID: "ws-teammate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteStore := newMockDeleteStore(workspaces...)

			err := runDelete(t, deleteStore, tt.workspaceName)

			require.NoError(t, err)
			assert.Equal(t, []string{tt.wantDeletedID}, deleteStore.deletedIDs)
		})
	}
}

func TestDeleteWorkspaceRefusesWhenTheTargetIsUnclear(t *testing.T) {
	tests := []struct {
		name       string
		workspaces []entity.Workspace
		wantErr    string
	}{
		{
			name: "two teammates' instances share the name",
			workspaces: []entity.Workspace{
				{ID: "ws-1", Name: "shared", CreatedByUserID: "user-a", Status: entity.Running},
				{ID: "ws-2", Name: "shared", CreatedByUserID: "user-b", Status: entity.Running},
			},
			wantErr: "multiple instances found with name shared",
		},
		{
			name: "a failed and a running instance share the name",
			workspaces: []entity.Workspace{
				{ID: "ws-1", Name: "shared", CreatedByUserID: "user-a", Status: entity.Failure},
				{ID: "ws-2", Name: "shared", CreatedByUserID: "user-b", Status: entity.Running},
			},
			wantErr: "multiple instances found with name shared",
		},
		{
			name:       "no instance in the org has the name",
			workspaces: []entity.Workspace{{ID: "ws-1", Name: "other", CreatedByUserID: testTeammateID}},
			wantErr:    "instance with id/name shared not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteStore := newMockDeleteStore(tt.workspaces...)

			err := runDelete(t, deleteStore, "shared")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Empty(t, deleteStore.deletedIDs)
		})
	}
}

func TestDeleteWorkspaceLetsBrevAdminsDeleteByIDOutsideTheOrg(t *testing.T) {
	deleteStore := newMockDeleteStore()
	deleteStore.user.GlobalUserType = entity.Admin

	err := runDelete(t, deleteStore, "ws-other-org")

	require.NoError(t, err)
	assert.Equal(t, []string{"ws-other-org"}, deleteStore.deletedIDs)
}

func TestDeleteWorkspacePreservesLookupErrors(t *testing.T) {
	for _, userType := range []entity.GlobalUserType{entity.Standard, entity.Admin} {
		for _, lookup := range []string{"user", "org"} {
			t.Run(string(userType)+"/"+lookup, func(t *testing.T) {
				deleteStore := newMockDeleteStore()
				deleteStore.user.GlobalUserType = userType
				lookupErr := errors.New("lookup dependency not found")
				wantOrgLookups := 0
				if lookup == "user" {
					deleteStore.userLookupErr = lookupErr
				} else {
					deleteStore.orgLookupErr = lookupErr
					wantOrgLookups = 1
				}

				err := runDelete(t, deleteStore, "gpu")

				require.ErrorIs(t, err, lookupErr)
				assert.Equal(t, wantOrgLookups, deleteStore.orgLookupCalls)
				assert.Empty(t, deleteStore.deletedIDs)
			})
		}
	}
}

func TestDeleteWorkspaceAdminCannotBypassAmbiguityWithNotFoundInName(t *testing.T) {
	deleteStore := newMockDeleteStore(
		entity.Workspace{ID: "ws-1", Name: "not found", CreatedByUserID: "user-a"},
		entity.Workspace{ID: "ws-2", Name: "not found", CreatedByUserID: "user-b"},
	)
	deleteStore.user.GlobalUserType = entity.Admin

	err := runDelete(t, deleteStore, "not found")

	require.ErrorContains(t, err, "multiple instances found")
	assert.Empty(t, deleteStore.deletedIDs)
}

func TestDeleteWorkspaceWithAPIKeyDoesNotFallBack(t *testing.T) {
	deleteStore := newMockDeleteStore()
	deleteStore.tokens = &entity.AuthTokens{APIKey: auth.BrevAPIKeyPrefix + "test-key"}
	deleteStore.user.GlobalUserType = entity.Admin

	err := runDelete(t, deleteStore, "ws-other-org")

	require.Error(t, err)
	assert.Empty(t, deleteStore.deletedIDs)
}

func TestDeleteWorkspaceBlocksTheSameStatusesAsTheConsole(t *testing.T) {
	tests := []struct {
		envStatus devplanev1.EnvironmentStatus
		wantErr   string
	}{
		{devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PENDING, "is still pending"},
		{devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PREPARING, "is still preparing"},
		{devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING, "is still provisioning"},
		{devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATING, "is already being deleted"},
		{devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_TERMINATED, "is already being deleted"},
		{devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_BUILDING, ""},
		{devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING, ""},
		{devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_STOPPED, ""},
		{devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_PROVISIONING_FAILED, ""},
	}
	for _, tt := range tests {
		t.Run(tt.envStatus.String(), func(t *testing.T) {
			// The instance's own status says deploying, so the deletable cases show that the
			// dev-plane status wins.
			deleteStore := newMockDeleteStore(entity.Workspace{ID: "ws-1", Name: "gpu", CreatedByUserID: testUserID, Status: entity.Deploying})
			deleteStore.environments["ws-1"].Status = tt.envStatus

			err := runDelete(t, deleteStore, "gpu")

			requireDeleteOutcome(t, deleteStore, err, tt.wantErr)
		})
	}
}

func TestDeleteWorkspaceFallsBackToTheInstanceStatus(t *testing.T) {
	tests := []struct {
		status  string
		wantErr string
	}{
		{entity.Deploying, "is still deploying"},
		{entity.Deleting, "is already being deleted"},
		{entity.Running, ""},
		{entity.Failure, ""},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			deleteStore := newMockDeleteStore(entity.Workspace{ID: "ws-1", Name: "gpu", CreatedByUserID: testUserID, Status: tt.status})
			// no dev-plane status, so the instance's own status decides
			deleteStore.environments["ws-1"].Status = devplanev1.EnvironmentStatus_ENVIRONMENT_STATUS_UNSPECIFIED

			err := runDelete(t, deleteStore, "gpu")

			requireDeleteOutcome(t, deleteStore, err, tt.wantErr)
		})
	}
}

// requireDeleteOutcome checks that instance ws-1 was deleted when wantErr is empty, and that
// nothing was deleted otherwise.
func requireDeleteOutcome(t *testing.T, deleteStore *mockDeleteStore, err error, wantErr string) {
	t.Helper()
	if wantErr == "" {
		require.NoError(t, err)
		assert.Equal(t, []string{"ws-1"}, deleteStore.deletedIDs)
		return
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), wantErr)
	assert.Empty(t, deleteStore.deletedIDs)
}
