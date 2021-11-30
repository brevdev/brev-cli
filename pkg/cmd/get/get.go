// Package get is for the get command
// TODO: delete this file if getmeprivatekeys isn't needed
package get

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

func getMe() brevapi.User {
	client, err := brevapi.NewDeprecatedCommandClient()
	if err != nil {
		panic(err)
	}
	user, err := client.GetMe()
	if err != nil {
		panic(err)
	}
	return *user
}

func getMeKeys() (*brevapi.UserKeys, error) {
	client, err := brevapi.NewDeprecatedCommandClient()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	keys, err := client.GetMeKeys()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return keys, nil
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
				return breverrors.WrapAndTrace(err)
			}

			// _, err = brevapi.CheckOutsideBrevErrorMessage(t)
			return breverrors.WrapAndTrace(err)
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
			t.Vprintf("User ID: %s", me.ID)
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
			meKeys, err := getMeKeys()
			if err != nil {
				t.Eprint(err.Error())
			}

			clusterID := config.GlobalConfig.GetDefaultClusterID()

			clusterKeys, err := meKeys.GetWorkspaceGroupKeysByGroupID(clusterID)
			if err != nil {
				t.Eprint(err.Error())
			}

			err = files.OverwriteString(files.GetCertFilePath(), clusterKeys.Cert)
			if err != nil {
				t.Eprint(err.Error())
			}
			err = files.OverwriteString(files.GetSSHPrivateKeyFilePath(), meKeys.PrivateKey)
			if err != nil {
				t.Eprint(err.Error())
			}
			return nil
		},
	}

	return cmd
}
