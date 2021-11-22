// Package secret lets you add secrests. Language Go makes you write extra.
package sshkeys

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

func GetAllWorkspaces(orgID string) ([]brevapi.Workspace, error) {
	client, err := brevapi.NewCommandClient()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	wss, err := client.GetWorkspaces(orgID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return wss, nil
}

type SecretStore interface {
	GetSSHKeys() (string, error)
}

func NewCmdSSHKeys(t *terminal.Terminal) *cobra.Command {


	cmd := &cobra.Command{
		Annotations: map[string]string{"housekeeping": ""},
		Use:         "ssh-key",
		Short:       "Get your pulic SSH-Key",
		Long:        "Get your pulic SSH-Key to add to pull and push from your git repository.",
		Example: `brev ssh-key`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := getSSHKeys(t)
			if err != nil {
				t.Vprintf(t.Red(err.Error()))
			}
			return nil
		},
	}

	return cmd
}

func getSSHKeys(t *terminal.Terminal) error {

	client, err := brevapi.NewCommandClient()
	if err != nil {
		return err
	}

	me, err := client.GetMe()
	if err != nil {
		return err
	}


	t.Eprint(t.Yellow("\nCreate an SSH Key. Copy your public key: \n"))
	t.Vprintf(me.PublicKey)
	t.Eprintf(t.Yellow("\n\nHere for Github: https://github.com/settings/keys"))
	t.Eprintf(t.Yellow("\nHere for Gitlab: https://gitlab.com/-/profile/keys\n"))




	return nil
}
