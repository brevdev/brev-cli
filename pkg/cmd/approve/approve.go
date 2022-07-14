package approve

import (
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	approveDescriptionShort = "Brev admin approve user"
	approveDescriptionLong  = "Brev admin approve user"
	approveExample          = "brev approve <user id>"
)

type ApproveStore interface {
	ApproveUserByID(userID string) (*entity.User, error)
}

func NewCmdApprove(t *terminal.Terminal, approveStore ApproveStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "approve",
		DisableFlagsInUseLine: true,
		Short:                 approveDescriptionShort,
		Long:                  approveDescriptionLong,
		Example:               approveExample,
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := approveUser(args[0], t, approveStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func approveUser(userID string, _ *terminal.Terminal, approveStore ApproveStore) error {
	_, err := approveStore.ApproveUserByID(userID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
