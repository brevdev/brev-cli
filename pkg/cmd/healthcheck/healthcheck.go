// Package healthcheck checks the health of the backend
package healthcheck

import (
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type HealthcheckStore interface {
	Healthcheck() error
}

func NewCmdHealthcheck(t *terminal.Terminal, store HealthcheckStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"housekeeping": ""},
		Use:         "healthcheck",
		Short:       "Check backend to see if it's healthy",
		Long:        "Check backend to see if it's healthy",
		Example:     `brev healthcheck`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := healthcheck(t, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func healthcheck(t *terminal.Terminal, store HealthcheckStore) error {
	err := store.Healthcheck()
	if err != nil {
		t.Print("Not Healthy!")
		return breverrors.WrapAndTrace(err)
	}
	t.Print("Healthy!")
	return nil
}
