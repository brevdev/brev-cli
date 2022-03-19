// Package logout is for the logout command
package logout

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/vpn"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type LogoutOptions struct {
	auth  Auth
	store LogoutStore
}

type Auth interface {
	Logout() error
}

type LogoutStore interface {
	vpn.ServiceMeshStore
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
		Args:                  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.RunLogout())
		},
	}
	return cmd
}

func (o *LogoutOptions) Complete(_ *cobra.Command, _ []string) error {
	return nil
}

func (o *LogoutOptions) RunLogout() error {
	workspaceID, err := o.store.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		fmt.Println("can not logout of workspace")
		return nil
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

	err = vpn.ResetVPN(o.store)
	if err != nil {
		allErr = multierror.Append(err)
	}

	if allErr != nil {
		return breverrors.WrapAndTrace(allErr)
	}
	return nil
}
