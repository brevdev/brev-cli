// Package get is for the get command
package get

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var sshLong = "dsfsdfds"

var sshExample = "dsfsdfsfd"

type SshOptions struct{}

func NewCmdGet(t *terminal.Terminal) *cobra.Command {
	// opts := SshOptions{}

	cmd := &cobra.Command{
		Use:         "get",
		// Annotations: map[string]string{"project": ""},
		Short:       "Get stuff",
		Long:        "Get stuff but longer.",
		Example:     `brev get [stuff]`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			t.Vprint(t.Green("heeyyy"))
			// open(t, orgName, proj)
			return nil
		},
	}

	// cmd := &cobra.Command{
	// 	Use:                   "ssh TYPE/NAME IDENTIFIER",
	// 	DisableFlagsInUseLine: true,
	// 	Short:                 "Get a resource by its identifer",
	// 	Long:                  sshLong,
	// 	Example:               sshExample,
	// 	// Args: func(cmd *cobra.Command, args []string) error {
	// 	// 	if len(args) <1{
	// 	// 		return fmt.Errorf("not enough args")
	// 	// 	} else {
	// 	// 		return nil
	// 	// 	}

	// 	// },
	// 	Args: cobra.MinimumNArgs(1),
	// 	// ValidArgsFunction: util.ResourceNameCompletionFunc(f, "pod"),
	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		cmdutil.CheckErr(opts.Complete(cmd, args))
	// 		cmdutil.CheckErr(opts.Validate(cmd, args))
	// 		cmdutil.CheckErr(opts.RunSSH(cmd, args))
	// 	},
	// }
	return cmd
}

func (o *SshOptions) Complete(cmd *cobra.Command, args []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *SshOptions) Validate(cmd *cobra.Command, args []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *SshOptions) RunSSH(cmd *cobra.Command, args []string) error {
	for _, arg := range args {
		fmt.Println(arg)
	}

	portforward.RunPortForwardFromCommand(
		"workspaceNamespace", // TODO find me with info from args(?)
		"workspacePodName",   // TODO findme
		[]string{"22", "2022", "2222"},
		"keyfileString",
		"certfileString",
	)
	return nil
}
