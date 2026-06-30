package credit

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCreditStore struct {
	org     *entity.Organization
	profile *store.BillingProfile
	orgs    []entity.Organization
}

func (f fakeCreditStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return f.org, nil
}

func (f fakeCreditStore) GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error) {
	return f.orgs, nil
}

func (f fakeCreditStore) GetCurrentUser() (*entity.User, error) {
	return nil, nil
}

func (f fakeCreditStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return nil, nil
}

func (f fakeCreditStore) GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error) {
	return nil, nil
}

func (f fakeCreditStore) GetBillingProfile(organizationID string) (*store.BillingProfile, error) {
	return f.profile, nil
}

func TestRunCreditUsesActiveOrgBalance(t *testing.T) {
	org := &entity.Organization{ID: "org-1", Name: "test-org"}
	remaining := int64(500)
	profile := &store.BillingProfile{CreditDetails: &store.CreditDetails{RemainingCredits: &remaining}}
	store := fakeCreditStore{org: org, profile: profile}

	err := RunCredit(terminal.New(), store, "")
	require.NoError(t, err)
	assert.Equal(t, org, store.org)
	assert.NotNil(t, store.profile)
	assert.NotNil(t, store.profile.CreditDetails)
	assert.Equal(t, remaining, *store.profile.CreditDetails.RemainingCredits)
}

func TestRunCreditUsesNamedOrg(t *testing.T) {
	org := &entity.Organization{ID: "org-2", Name: "named-org"}
	remaining := int64(1234)
	profile := &store.BillingProfile{CreditDetails: &store.CreditDetails{RemainingCredits: &remaining}}
	store := fakeCreditStore{org: org, profile: profile, orgs: []entity.Organization{*org}}

	err := RunCredit(terminal.New(), store, "named-org")
	require.NoError(t, err)
	assert.Equal(t, org, store.org)
	assert.NotNil(t, store.profile)
	assert.Equal(t, remaining, *store.profile.CreditDetails.RemainingCredits)
}

func TestRunCreditFailsWhenCreditBalanceMissing(t *testing.T) {
	org := &entity.Organization{ID: "org-1", Name: "test-org"}

	tests := []struct {
		name    string
		profile *store.BillingProfile
	}{
		{name: "billing profile missing", profile: nil},
		{name: "credit details missing", profile: &store.BillingProfile{CreditDetails: nil}},
		{name: "remaining credits missing", profile: &store.BillingProfile{CreditDetails: &store.CreditDetails{RemainingCredits: nil}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := fakeCreditStore{org: org, profile: tt.profile}

			err := RunCredit(terminal.New(), store, "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to retrieve credit balance")
		})
	}
}
