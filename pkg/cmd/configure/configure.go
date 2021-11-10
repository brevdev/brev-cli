// Package configure is for the configure command
//
// ssh host file format:
//
// 	Host <workspace-name>
// 		Hostname 0.0.0.0
// 		IdentityFile ~/.brev/brev.pem
//		User brev
//		Port <some-available-port>
//
// also think that file stuff should probably live in files package
package configure

import (
	"github.com/brevdev/brev-cli/pkg/brev_ssh"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type configureOptions struct{}

func NewCmdConfigure() *cobra.Command {
	opts := configureOptions{}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "configure",
		DisableFlagsInUseLine: true,
		Short:                 "configure ssh for brev",
		Long:                  "configure ssh for brev",
		Example:               "brev configure",
		Args:                  cobra.NoArgs,
		// ValidArgsFunction: util.ResourceNameCompletionFunc(f, "pod"),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate(cmd, args))
			cmdutil.CheckErr(opts.RunConfigure(cmd, args))
		},
	}
	return cmd
}

func (o *configureOptions) Complete(cmd *cobra.Command, args []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *configureOptions) Validate(cmd *cobra.Command, args []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *configureOptions) RunConfigure(cmd *cobra.Command, args []string) error {
	return brev_ssh.ConfigureSSH()
}
