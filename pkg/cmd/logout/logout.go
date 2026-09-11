// Package logout is for the logout command
package logout

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/auth"
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
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "logout",
		DisableFlagsInUseLine: true,
		Short:                 "Log out of Brev",
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

	// A child process can't clear an env var in the parent shell. If
	// BREV_API_KEY is set, saved credentials are gone but the env key still
	// authenticates every command, so surface that instead of pretending the
	// logout was complete. Unset in-process for this session.
	if strings.TrimSpace(os.Getenv(auth.APIKeyEnvVar)) != "" {
		fmt.Println("BREV_API_KEY may still be set in your shell. Run 'unset BREV_API_KEY' to fully log out.")
		_ = os.Unsetenv(auth.APIKeyEnvVar)
	}

	return nil
}
