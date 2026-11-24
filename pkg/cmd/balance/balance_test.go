package balance

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBalanceStore struct {
	org     *entity.Organization
	profile *store.BillingProfile
}

func (f fakeBalanceStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return f.org, nil
}

func (f fakeBalanceStore) GetBillingProfile(organizationID string) (*store.BillingProfile, error) {
	return f.profile, nil
}

func TestRunBalanceUsesActiveOrgBalance(t *testing.T) {
	org := &entity.Organization{ID: "org-1", Name: "test-org"}
	remaining := int64(500)
	profile := &store.BillingProfile{CreditDetails: &store.CreditDetails{RemainingCredits: &remaining}}
	balanceStore := fakeBalanceStore{org: org, profile: profile}

	err := RunBalance(terminal.New(), balanceStore)
	require.NoError(t, err)
	assert.Equal(t, org, balanceStore.org)
	assert.NotNil(t, balanceStore.profile)
	assert.NotNil(t, balanceStore.profile.CreditDetails)
	assert.Equal(t, remaining, *balanceStore.profile.CreditDetails.RemainingCredits)
}

func TestRunBalanceFailsWhenCreditBalanceMissing(t *testing.T) {
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
			balanceStore := fakeBalanceStore{org: org, profile: tt.profile}

			err := RunBalance(terminal.New(), balanceStore)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to retrieve credit balance")
		})
	}
}
