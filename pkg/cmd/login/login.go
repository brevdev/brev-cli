// Package login is for the get command
package login

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type LoginOptions struct{}

type LoginStore interface {
	CreateUser(idToken string) (*brevapi.User, error)
}

// func NewCmdLogin() *cobra.Command {
func NewCmdLogin(loginStore LoginStore) *cobra.Command {
	opts := LoginOptions{}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "login",
		DisableFlagsInUseLine: true,
		Short:                 "log into brev",
		Long:                  "log into brev",
		Example:               "brev login",
		Args:                  cobra.NoArgs,
		// ValidArgsFunction: util.ResourceNameCompletionFunc(f, "pod"),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate(cmd, args))
			cmdutil.CheckErr(opts.RunLogin(cmd, args, loginStore))
		},
	}
	return cmd
}

func (o *LoginOptions) Complete(_ *cobra.Command, _ []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *LoginOptions) Validate(_ *cobra.Command, _ []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *LoginOptions) RunLogin(_ *cobra.Command, _ []string, loginStore LoginStore) error {
	// func (o *LoginOptions) RunLogin(cmd *cobra.Command, args []string) error {
	token, err := brevapi.Login(false)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = postLogin(*token, loginStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

/*
After logging in:
	> ensure the user is created /Me or create the user
	> ensure there's an org, or create the first one "username-hq"
	>
*/
func postLogin(token string, loginStore LoginStore) error {
	// TODO: hit GetMe and if fails create user
	client, err := brevapi.NewCommandClient()
	if err != nil {
		fmt.Println("error on client")
		return err
	}
	_, err = client.GetMe()
	if err != nil {
		fmt.Println("error on getting ME")
		// TODO: if the error is not a network call create the account
		// _, err := client.CreateUser(creds.IDToken)
		_, err = loginStore.CreateUser(token)

		if err != nil {
			fmt.Println("error on creating user")
			return err
		}
		fmt.Println("created user")

	}
	return nil
}
