package credit

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type CreditStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetBillingProfile(organizationID string) (*store.BillingProfile, error)
	completions.CompletionStore
}

func NewCmdCredit(t *terminal.Terminal, creditStore CreditStore, noCreditStore CreditStore) *cobra.Command {
	var orgFlag string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"organization": ""},
		Use:                   "credit",
		DisableFlagsInUseLine: true,
		Short:                 "Show active organization's credit balance",
		Long:                  "Print the credit balance for the active organization.",
		Example: `
  brev credit
  brev credit --org myorg
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
			err := RunCredit(t, creditStore, orgFlag)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&orgFlag, "org", "o", "", "organization (will override active org)")
	err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noCreditStore, t))
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err))
		fmt.Print(breverrors.WrapAndTrace(err))
	}

	return cmd
}

func RunCredit(t *terminal.Terminal, creditStore CreditStore, orgFlag string) error {
	var org *entity.Organization
	if orgFlag != "" {
		orgs, err := creditStore.GetOrganizations(&store.GetOrganizationsOptions{Name: orgFlag})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if len(orgs) == 0 {
			return fmt.Errorf("no org found with name %s", orgFlag)
		} else if len(orgs) > 1 {
			return fmt.Errorf("more than one org found with name %s", orgFlag)
		}

		org = &orgs[0]
	} else {
		currOrg, err := creditStore.GetActiveOrganizationOrDefault()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if currOrg == nil {
			return fmt.Errorf("no orgs exist")
		}
		org = currOrg
	}

	profile, err := creditStore.GetBillingProfile(org.ID)
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
