// Package secret lets you add secrests. Language Go makes you write extra.
package secret

import (
	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// Helper functions.
func getMe() brevapi.User {
	client, err := brevapi.NewCommandClient()
	if err != nil {
		panic(err)
	}
	user, err := client.GetMe()
	if err != nil {
		panic(err)
	}
	return *user
}

func getOrgs() ([]brevapi.Organization, error) {
	client, err := brevapi.NewCommandClient()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	orgs, err := client.GetOrgs()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return orgs, nil
}

func GetAllWorkspaces(orgID string) ([]brevapi.Workspace, error) {
	client, err := brevapi.NewCommandClient()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	wss, err := client.GetWorkspaces(orgID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return wss, nil
}

func NewCmdSecret(t *terminal.Terminal) *cobra.Command {
	var envtype string
	var name string
	var value string
	var path string

	cmd := &cobra.Command{
		Annotations: map[string]string{"housekeeping": ""},
		Use:         "secret",
		Short:       "Add a secret/environment variable",
		Long:        "Add a secret/environment variable to your workspace, all workspaces in an org, or all of your workspaces",
		Example: `
  brev secret --name naaamme --value vaaalluueee --type [file, env-var] --file-path 
  brev secret --name SERVER_URL --value https://brev.sh --type env-var
  brev secret --name AWS_KEY --value ... --type file --file-path 
		`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		// Args:      cobra.MinimumNArgs(0),
		// ValidArgs: []string{"orgs", "workspaces"},
		RunE: func(cmd *cobra.Command, args []string) error {
			addSecret(t, envtype, name, value, path)
			return nil
		},
	}

	cmd.Flags().StringVarP(&envtype, "type", "t", "", "type of secret (env var or file)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "name of environment variable or secret file")
	cmd.Flags().StringVarP(&value, "value", "v", "", "value of environment variable or secret file")
	cmd.Flags().StringVarP(&path, "file-path", "p", "", "file path (if secret file)")

	err := cmd.RegisterFlagCompletionFunc("type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"file", "env-var"}, cobra.ShellCompDirectiveNoSpace
	})
	if err != nil {
		t.Errprint(err, "cli err")
	}

	return cmd
}

func addSecret(t *terminal.Terminal, envtype string, name string, value string, path string) {

	if envtype=="" || name=="" || value=="" || path == "" {
		t.Vprintf(t.Yellow("\nSome flags omitted, running interactive mode!\n"))
	}

	if name == "" {
		name = brevapi.PromptGetInput(brevapi.PromptContent{
			Label:    "Environment variable/secret name: ",
			ErrorMsg: "error",
		})
	}
}
