package redeem

import (
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type RedeemStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	RedeemCouponCode(organizationID string, code string) (*store.RedeemCouponCodeResponse, error)
}

func NewCmdRedeem(t *terminal.Terminal, redeemStore RedeemStore) *cobra.Command {
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
			err := RunRedeem(t, redeemStore, args[0])
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func RunRedeem(t *terminal.Terminal, redeemStore RedeemStore, code string) error {
	startTime := time.Now()

	org, err := redeemStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return breverrors.NewValidationError("no orgs exist")
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
