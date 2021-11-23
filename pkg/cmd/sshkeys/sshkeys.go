// Package sshkeys gets your public ssh key to add to github/gitlab
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
			err := brevapi.GetandDisplaySSHKeys(t)
			if err != nil {
				t.Vprintf(t.Red(err.Error()))
			}
			return nil
		},
	}

	return cmd
}
