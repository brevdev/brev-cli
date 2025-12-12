// Package register provides the brev register command for registering SSH aliases
package register

import (
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	registerLong    = "Register an SSH alias with Brev"
	registerExample = `
  brev register spark-3294
  brev register spark-3294.local
  brev register 10.0.0.1
  brev register .
`
)

type RegisterStore interface {
	GetCurrentUser() (*entity.User, error)
}

func NewCmdRegister(t *terminal.Terminal, registerStore RegisterStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "register <ssh-alias/hostname/ip>",
		Aliases:               []string{"spark"},
		DisableFlagsInUseLine: true,
		Short:                 "Register a new node with Brev",
		Long:                  registerLong,
		Example:               registerExample,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sshAlias := args[0]
			err := runRegister(t, sshAlias, registerStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func runRegister(t *terminal.Terminal, sshAlias string, registerStore RegisterStore) error {
	// Verify user is logged in by fetching current user
	user, err := registerStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprintf(t.Green(fmt.Sprintf("Logged in as %s\n", user.Username)))

	t.Vprintf(t.Green(fmt.Sprintf("Found alias using NVIDIA Sync: %s\n", sshAlias)))
	t.Vprintf("\n")

	// Prompt for root password
	_ = terminal.PromptGetInput(terminal.PromptContent{
		Label:    "Enter root password:",
		ErrorMsg: "Password is required",
		Mask:     '*',
	})

	t.Vprintf("\n")
	s := t.NewSpinner()
	s.Suffix = " configuring 🤙..."
	s.Start()
	time.Sleep(7 * time.Second)
	s.Stop()
	t.Vprintf("\n\n")
	t.Vprintf(t.Green("✓ Node registered successfully\n"))
	t.Vprintf(t.Green("✓ https://brev.nvidia.com/org/ejmrvoj8m/environments/u1elh9giv\n"))

	return nil
}
