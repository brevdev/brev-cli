// Package signup is login with credit card + specific auth0 url
package signup

import (
	l "github.com/brevdev/brev-cli/pkg/cmd/login"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// loginStore must be a no prompt store
func NewCmdSignup(t *terminal.Terminal, loginStore l.LoginStore, auth l.Auth) *cobra.Command {
	opts := l.LoginOptions{
		Auth:       auth,
		LoginStore: loginStore,
	}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "signup",
		DisableFlagsInUseLine: true,
		Short:                 "Signup for brev",
		Long:                  "Signup for brev",
		Example:               "brev signup",
		Args:                  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(t, cmd, args))
			cmdutil.CheckErr(opts.RunLogin(t))
		},
	}
	return cmd
}
