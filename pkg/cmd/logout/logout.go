// Package logout is for the logout command
package logout

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type LogoutOptions struct {
	auth  Auth
	store LogoutStore
}

type Auth interface {
	Logout() error
}

type LogoutStore interface {
	ClearDefaultOrganization() error
	GetCurrentWorkspaceID() (string, error)
}

func NewCmdLogout(auth Auth, store LogoutStore) *cobra.Command {
	opts := LogoutOptions{
		auth:  auth,
		store: store,
	}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "logout",
		DisableFlagsInUseLine: true,
		Short:                 "Log out of brev",
		Long:                  "Log out of brev by deleting the credential file",
		Example:               "brev logout",
		Args:                  cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := opts.RunLogout()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func (o *LogoutOptions) RunLogout() error {
	workspaceID, err := o.store.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		return fmt.Errorf("can not logout of workspace")
	}

	// best effort
	var allErr error
	err = o.auth.Logout()
	if err != nil {
		if !strings.Contains(err.Error(), ".brev/credentials.json: no such file or directory") {
			allErr = multierror.Append(err)
		}
	}

	err = o.store.ClearDefaultOrganization()
	if err != nil {
		allErr = multierror.Append(err)
	}

	if allErr != nil {
		return breverrors.WrapAndTrace(allErr)
	}
	return nil
}
