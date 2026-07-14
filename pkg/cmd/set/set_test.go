package set

import (
	"strings"
	"testing"

	authpkg "github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
)

const testAPIKey = authpkg.BrevAPIKeyPrefix + "test-key"

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

func TestSetRejectsAPIKeyAuth(t *testing.T) {
	s := &mockSetStore{
		authTokens: &entity.AuthTokens{APIKey: testAPIKey, APIKeyOrgID: "org-test"},
		orgs:       []entity.Organization{{ID: "org-other", Name: "other-org"}},
	}

	err := set("other-org", s)

	if err == nil {
		t.Fatal("expected API key auth set error, got nil")
	}
	if !strings.Contains(err.Error(), "api key auth is scoped") {
		t.Fatalf("expected API key auth validation error, got %v", err)
	}
	if s.getOrganizations != 0 {
		t.Fatalf("expected set to skip org lookup, got %d calls", s.getOrganizations)
	}
	if s.setDefaultOrgCalls != 0 {
		t.Fatalf("expected set to skip default org write, got %d calls", s.setDefaultOrgCalls)
	}
}
