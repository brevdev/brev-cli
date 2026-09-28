// Package invite is for inviting
package invite

import (
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type InviteStore interface {
	GetUsers(queryParams map[string]string) ([]entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	CreateInviteLink(organizationID string) (string, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
}

func NewCmdInvite(t *terminal.Terminal, loginInviteStore InviteStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"organization": ""},
		Use:         "invite",
		Short:       "Generate an invite link (alias for 'brev org invite')",
		Long:        "Get an invite link to your active org. Use the optional org flag to invite to a different org",
		Example: `
  brev invite
  brev invite --org <orgid>
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
			err := RunInvite(t, loginInviteStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func RunInvite(t *terminal.Terminal, inviteStore InviteStore) error {
	org, err := inviteStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return breverrors.NewValidationError("no orgs exist")
	}

	token, err := inviteStore.CreateInviteLink(org.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("Share this link to add someone to %s. It will expire in 7 days.", t.Green(org.Name))
	t.Vprintf("\n\n  %s", t.Green("▸"))
	t.Vprintf("    %s", t.White("%s/invite?token=%s\n\n", config.GlobalConfig.GetConsoleURL(), token))

	return nil
}
