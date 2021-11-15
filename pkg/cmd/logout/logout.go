// Package logout is for the logout command
package logout

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type LogoutOptions struct{}

func NewCmdLogout() *cobra.Command {
	opts := LogoutOptions{}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "logout",
		DisableFlagsInUseLine: true,
		Short:                 "log out of brev",
		Long:                  "log out of brev by deleting the credential file",
		Example:               "brev logout",
		Args:                  cobra.NoArgs,
		// ValidArgsFunction: util.ResourceNameCompletionFunc(f, "pod"),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate(cmd, args))
			cmdutil.CheckErr(opts.RunLogout(cmd, args))
		},
	}
	return cmd
}

func (o *LogoutOptions) Complete(_ *cobra.Command, _ []string) error {
	// func (o *LogoutOptions) Complete(cmd *cobra.Command, args []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *LogoutOptions) Validate(_ *cobra.Command, _ []string) error {
	// func (o *LogoutOptions) Validate(cmd *cobra.Command, args []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *LogoutOptions) RunLogout(_ *cobra.Command, _ []string) error {
	// func (o *LogoutOptions) RunLogout(cmd *cobra.Command, args []string) error {
	err := brevapi.Logout()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
