// Package login is for the get command
package login

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type LoginOptions struct{}

func NewCmdLogin() *cobra.Command {
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
			cmdutil.CheckErr(opts.RunLogin(cmd, args))
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

func (o *LoginOptions) RunLogin(_ *cobra.Command, _ []string) error {
	// func (o *LoginOptions) RunLogin(cmd *cobra.Command, args []string) error {
	err := brevapi.Login(false)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
