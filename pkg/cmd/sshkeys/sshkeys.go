// Package sshkeys gets your public ssh key to add to github/gitlab
package sshkeys

import (
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type SSHKeyStore interface {
	GetCurrentUser() (*entity.User, error)
}

func NewCmdSSHKeys(t *terminal.Terminal, sshKeyStore SSHKeyStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"housekeeping": ""},
		Use:         "ssh-key",
		Short:       "Get your pulic SSH-Key",
		Long:        "Get your pulic SSH-Key to add to pull and push from your git repository.",
		Example:     `brev ssh-key`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			user, err := sshKeyStore.GetCurrentUser()
			if err != nil {
				t.Vprintf(t.Red(err.Error()))
			}
			terminal.DisplaySSHKeys(t, user.PublicKey)
			return nil
		},
	}

	return cmd
}
