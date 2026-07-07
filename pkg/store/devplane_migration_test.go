package store

import (
	"context"
	"testing"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

type mockDevPlaneUserService struct {
	currentUser      *nodev1.User
	publicSSHKey     string
	keys             *entity.UserKeys
	getUser          *nodev1.User
	gotUserID        string
	gotBlockedUserID string
	gotBlocked       bool
}

func (m *mockDevPlaneUserService) GetCurrentUser(context.Context) (*nodev1.User, string, error) {
	return m.currentUser, m.publicSSHKey, nil
}

func (m *mockDevPlaneUserService) GetCurrentUserKeys(context.Context) (*entity.UserKeys, error) {
	return m.keys, nil
}

func (m *mockDevPlaneUserService) GetUser(_ context.Context, userID string) (*nodev1.User, error) {
	m.gotUserID = userID
	return m.getUser, nil
}

func (m *mockDevPlaneUserService) SetUserBlocked(_ context.Context, userID string, blocked bool) (*nodev1.User, error) {
	m.gotBlockedUserID = userID
	m.gotBlocked = blocked
	return m.getUser, nil
}

type mockDevPlaneOrganizationService struct {
	organizations []*nodev1.Organization
	organization  *nodev1.Organization
	gotIDs        []string
	gotID         string
}

func (m *mockDevPlaneOrganizationService) ListOrganizations(_ context.Context, ids []string) ([]*nodev1.Organization, error) {
	m.gotIDs = ids
	return m.organizations, nil
}

func (m *mockDevPlaneOrganizationService) GetOrganization(_ context.Context, id string) (*nodev1.Organization, error) {
	m.gotID = id
	return m.organization, nil
}

func TestIdentityStoresUseDevPlaneServices(t *testing.T) {
	metadata, err := structpb.NewStruct(map[string]any{
		"baseWorkspaceRepo": "github.com/alice/base",
		"globalUserType":    "Admin",
		"onboardingData": map[string]any{
			"usedCLI": true,
		},
	})
	require.NoError(t, err)
	apiUser := &nodev1.User{
		UserId:       "user-1",
		Username:     "alice",
		DisplayName:  "Alice",
		DefaultEmail: "alice@example.com",
		PlatformRole: "user",
		Metadata:     metadata,
		ExternalIdentities: []*nodev1.ExternalIdentity{{
			IdentityId: "identity-1",
			ExternalId: "external-1",
			Provider:   "kas",
		}},
	}
	users := &mockDevPlaneUserService{
		currentUser:  apiUser,
		publicSSHKey: "ssh-rsa public",
		keys:         &entity.UserKeys{PublicKey: "public", PrivateKey: "private"},
		getUser:      apiUser,
	}
	store := MakeMockAuthHTTPStore().withDevPlaneServices(&devPlaneServices{user: users})

	current, err := store.GetCurrentUser()
	require.NoError(t, err)
	require.Equal(t, "user-1", current.ID)
	require.Equal(t, "Alice", current.Name)
	require.Equal(t, "alice@example.com", current.Email)
	require.Equal(t, "ssh-rsa public", current.PublicKey)
	require.Equal(t, "github.com/alice/base", current.BaseWorkspaceRepo)
	require.Equal(t, entity.Admin, current.GlobalUserType)
	require.Equal(t, true, current.OnboardingData["usedCLI"])

	keys, err := store.GetCurrentUserKeys()
	require.NoError(t, err)
	require.Equal(t, "private", keys.PrivateKey)

	target, err := store.GetUserByID("user-2")
	require.NoError(t, err)
	require.Equal(t, "user-1", target.ID)
	require.Equal(t, "user-2", users.gotUserID)

	require.NoError(t, store.BanUser("user-2"))
	require.Equal(t, "user-2", users.gotBlockedUserID)
	require.True(t, users.gotBlocked)
}

func TestOrganizationStoresUseCurrentUserAccessesAndDevPlane(t *testing.T) {
	users := &mockDevPlaneUserService{currentUser: &nodev1.User{
		UserId: "user-1",
		OrganizationAccesses: []*nodev1.OrganizationAccess{
			{OrganizationId: "org-1"},
			{OrganizationId: "org-2"},
		},
	}}
	organizations := &mockDevPlaneOrganizationService{
		organizations: []*nodev1.Organization{
			{OrganizationId: "org-1", DisplayName: "One"},
			{OrganizationId: "org-2", DisplayName: "Two"},
		},
		organization: &nodev1.Organization{OrganizationId: "org-2", DisplayName: "Two"},
	}
	store := MakeMockAuthHTTPStore().withDevPlaneServices(&devPlaneServices{
		user:         users,
		organization: organizations,
	})

	listed, err := store.GetOrganizations(nil)
	require.NoError(t, err)
	require.Equal(t, []entity.Organization{{ID: "org-1", Name: "One"}, {ID: "org-2", Name: "Two"}}, listed)
	require.Equal(t, []string{"org-1", "org-2"}, organizations.gotIDs)

	organization, err := store.GetOrganization("org-2")
	require.NoError(t, err)
	require.Equal(t, &entity.Organization{ID: "org-2", Name: "Two"}, organization)
	require.Equal(t, "org-2", organizations.gotID)
}
