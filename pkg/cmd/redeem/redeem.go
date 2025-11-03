package redeem

import (
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type RedeemStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	RedeemCouponCode(organizationID string, code string) (*store.RedeemCouponCodeResponse, error)
	completions.CompletionStore
}

func NewCmdRedeem(t *terminal.Terminal, redeemStore RedeemStore, noRedeemStore RedeemStore) *cobra.Command {
	var orgFlag string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"organization": ""},
		Use:                   "redeem <code>",
		DisableFlagsInUseLine: true,
		Short:                 "Redeem a code for credits",
		Long:                  "Redeem a code to add credits to your active organization",
		Example: `
  brev redeem ABC123XYZ
  brev redeem ABC123XYZ --org myorg
		`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		Args: cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunRedeem(t, redeemStore, args[0], orgFlag)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&orgFlag, "org", "o", "", "organization (will override active org)")
	err := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(noRedeemStore, t))
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err))
		fmt.Print(breverrors.WrapAndTrace(err))
	}

	return cmd
}

func RunRedeem(t *terminal.Terminal, redeemStore RedeemStore, code string, orgFlag string) error {
	startTime := time.Now()

	var org *entity.Organization
	if orgFlag != "" {
		orgs, err := redeemStore.GetOrganizations(&store.GetOrganizationsOptions{Name: orgFlag})
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
		currOrg, err := redeemStore.GetActiveOrganizationOrDefault()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if currOrg == nil {
			return fmt.Errorf("no orgs exist")
		}
		org = currOrg
	}

	result, err := redeemStore.RedeemCouponCode(org.ID, code)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	duration := time.Since(startTime)

	t.Vprint(t.Green(fmt.Sprintf("✓ Successfully redeemed code: %s\n", code)))
	if result.Data.Transaction.AmountUSD != "" {
		t.Vprintf("  Credits added: $%s\n", result.Data.Transaction.AmountUSD)
	}
	t.Vprintf("  Organization: %s\n", org.Name)
	t.Vprintf("  Duration: %v\n", duration.Round(time.Millisecond))

	return nil
}
