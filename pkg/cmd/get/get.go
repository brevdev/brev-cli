// Package get is for the get command
// TODO: delete this file if getmeprivatekeys isn't needed
package get

import (
	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

func getMe() brev_api.User {
	client, _ := brev_api.NewClient()
	user, _ := client.GetMe()
	return *user
}

func getMePrivateKeys() (*brev_api.PrivateKeys, error) {
	client, err := brev_api.NewClient()
	if err != nil {
		return nil, err
	}
	return client.GetMePrivateKeys()
}

func NewCmdGet(t *terminal.Terminal) *cobra.Command {
	// opts := SshOptions{}

	cmd := &cobra.Command{
		Use: "get",
		// Annotations: map[string]string{"project": ""},
		Short:   "Get stuff",
		Long:    "Get stuff but longer.",
		Example: `brev get [stuff]`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}

			// _, err = brev_api.CheckOutsideBrevErrorMessage(t)
			return err
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{}, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveDefault
		},
	}

	cmd.AddCommand(newCmdMe(t))
	cmd.AddCommand(newCmdMePrivateKeys(t))

	return cmd
}


func newCmdMe(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "me",
		Short: "return info about the current authenticated user",
		Long:  "return info about the current authenticated user",
		Example: `brev get me

		User ID: c0wj3ro`,
		RunE: func(cmd *cobra.Command, args []string) error {
			me := getMe()
			t.Vprintf("User ID: %s", me.Id)
			return nil
		},
	}

	return cmd
}

func newCmdMePrivateKeys(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sshprivkey",
		Short:   "get your ssh privatekey",
		Long:    "get your ssh privatekey and print it to stdout",
		Example: `brev get sshprivkey`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mePrivateKeys, err := getMePrivateKeys()
			if err != nil {
				t.Eprint(err.Error())
			}
			files.OverwriteString(files.GetCertFilePath(), mePrivateKeys.Cert)
			files.OverwriteString(files.GetSSHPrivateKeyFilePath(), mePrivateKeys.SSHPrivateKey)
			return nil
		},
	}

	return cmd
}
