package stop

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
)

func TestStopWorkspaceSelf(_ *testing.T) {
	// err := stopThisWorkspace(nil, nil)
	// assert.Nil(t, err)
}

type fakeStopStore struct {
	workspace  entity.Workspace
	stoppedIDs []string
}

func (f *fakeStopStore) GetAuthTokens() (*entity.AuthTokens, error) { return nil, nil }

func (f *fakeStopStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return &entity.Organization{ID: "org-1"}, nil
}

func (f *fakeStopStore) GetCurrentUser() (*entity.User, error) {
	return &entity.User{ID: "user-1"}, nil
}

func (f *fakeStopStore) GetWorkspaceByNameOrID(_ string, _ string) ([]entity.Workspace, error) {
	return []entity.Workspace{f.workspace}, nil
}

func (f *fakeStopStore) GetWorkspaces(_ string, _ *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	return []entity.Workspace{f.workspace}, nil
}

func (f *fakeStopStore) GetOrganizations(_ *store.GetOrganizationsOptions) ([]entity.Organization, error) {
	return nil, nil
}

func (f *fakeStopStore) StopWorkspace(workspaceID string) (*entity.Workspace, error) {
	f.stoppedIDs = append(f.stoppedIDs, workspaceID)
	return &f.workspace, nil
}

func (f *fakeStopStore) IsWorkspace() (bool, error) { return false, nil }

func (f *fakeStopStore) GetCurrentWorkspaceID() (string, error) { return "", nil }

func newFakeStopStore(status string) *fakeStopStore {
	return &fakeStopStore{workspace: entity.Workspace{
		ID:               "ws-1",
		Name:             "my-instance",
		Status:           status,
		CreatedByUserID:  "user-1",
		InstanceTypeInfo: &entity.InstanceTypeInfo{Stoppable: true},
	}}
}

func TestStopWorkspace_SkipsStopRequestWhenAlreadyStopped(t *testing.T) {
	s := newFakeStopStore(entity.Stopped)

	err := stopWorkspace("my-instance", terminal.New(), s, false)

	assert.NoError(t, err)
	assert.Empty(t, s.stoppedIDs)
}

func TestStopWorkspace_SendsStopRequestWhenRunning(t *testing.T) {
	s := newFakeStopStore(entity.Running)

	err := stopWorkspace("my-instance", terminal.New(), s, false)

	assert.NoError(t, err)
	assert.Equal(t, []string{"ws-1"}, s.stoppedIDs)
}
