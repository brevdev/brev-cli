package balance

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type BalanceStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetBillingProfile(organizationID string) (*store.BillingProfile, error)
}

func NewCmdBalance(t *terminal.Terminal, balanceStore BalanceStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"organization": ""},
		Use:                   "balance",
		DisableFlagsInUseLine: true,
		Short:                 "Show active organization's credit balance",
		Long:                  "Print the credit balance for the active organization.",
		Example: `
  brev balance
  brev balance --org myorg
        `,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		Args: cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunBalance(t, balanceStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func RunBalance(t *terminal.Terminal, balanceStore BalanceStore) error {
	org, err := balanceStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return fmt.Errorf("no orgs exist")
	}

	profile, err := balanceStore.GetBillingProfile(org.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if profile == nil || profile.CreditDetails == nil || profile.CreditDetails.RemainingCredits == nil {
		return fmt.Errorf("failed to retrieve credit balance")
	}

	remainingCents := *profile.CreditDetails.RemainingCredits
	dollars := float64(remainingCents) / 100.0

	t.Vprint(t.Green("✓ Retrieved credit balance\n"))
	t.Vprintf("  Organization: %s\n", org.Name)
	t.Vprintf("  ID: %s\n", org.ID)
	t.Vprintf("  Credits: $%.2f\n", dollars)

	return nil
}
