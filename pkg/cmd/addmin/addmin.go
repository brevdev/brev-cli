package addmin

import (
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

type Store interface {
	GetCurrentUser() (*entity.User, error)
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
}

func NewCmdAddmin(t *terminal.Terminal, store Store) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "addmin",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		// Ensure 1 argument
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := Run(t, args[0], store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func Run(t *terminal.Terminal, userID string, store Store) error {
	user, err := store.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if user.GlobalUserType != "Admin" {
		t.Vprint(t.Red("Access Denied. You must be an admin to run this command."))
		return breverrors.WrapAndTrace(err)
	}

	// update user to set type to "admin"
	updatedUser := &entity.UpdateUser{
		GlobalUserType: "Admin",
	}

	_, err = store.UpdateUser(userID, updatedUser)

	t.Vprint(t.Green("User updated to admin 🤙"))
	return nil
}
