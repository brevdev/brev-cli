package completions

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCompletionStore struct {
	activeOrg     *entity.Organization
	organizations []entity.Organization
	workspaces    map[string][]entity.Workspace
	requestedOrg  string
}

func (m *mockCompletionStore) GetAuthTokens() (*entity.AuthTokens, error) { return nil, nil }

func (m *mockCompletionStore) GetWorkspaces(orgID string, _ *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	m.requestedOrg = orgID
	return m.workspaces[orgID], nil
}

func (m *mockCompletionStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return m.activeOrg, nil
}

func (m *mockCompletionStore) GetCurrentUser() (*entity.User, error) {
	return &entity.User{ID: "user-1"}, nil
}

func (m *mockCompletionStore) GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error) {
	if options == nil || options.Name == "" {
		return m.organizations, nil
	}
	var matches []entity.Organization
	for _, org := range m.organizations {
		if org.Name == options.Name {
			matches = append(matches, org)
		}
	}
	return matches, nil
}

func TestWorkspaceCompletionUsesOrgOverride(t *testing.T) {
	completionStore := &mockCompletionStore{
		activeOrg:     &entity.Organization{ID: "org-a", Name: "orgA"},
		organizations: []entity.Organization{{ID: "org-a", Name: "orgA"}, {ID: "org-b", Name: "orgB"}},
		workspaces: map[string][]entity.Workspace{
			"org-a": {{Name: "instance-a"}},
			"org-b": {{Name: "instance-b"}},
		},
	}
	rootCmd := &cobra.Command{Use: "brev"}
	rootCmd.PersistentFlags().String("org", "", "organization")
	shellCmd := &cobra.Command{Use: "shell"}
	rootCmd.AddCommand(shellCmd)
	require.NoError(t, rootCmd.PersistentFlags().Set("org", "orgB"))

	names, directive := GetAllWorkspaceNameCompletionHandler(completionStore, terminal.New())(shellCmd, nil, "")

	assert.Equal(t, []string{"instance-b"}, names)
	assert.Equal(t, cobra.ShellCompDirectiveDefault, directive)
	assert.Equal(t, "org-b", completionStore.requestedOrg)
}
