// Package login is for the get command
package login

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type LoginOptions struct{}

type LoginStore interface {
	CreateUser(idToken string) (*brevapi.User, error)
	GetOrganizations() ([]brevapi.Organization, error)
	CreateOrganization(req store.CreateOrganizationRequest) (*brevapi.Organization, error)
}

func NewCmdLogin(t *terminal.Terminal, loginStore LoginStore) *cobra.Command {
	opts := LoginOptions{}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "login",
		DisableFlagsInUseLine: true,
		Short:                 "Log into brev",
		Long:                  "Log into brev",
		Example:               "brev login",
		Args:                  cobra.NoArgs,
		// ValidArgsFunction: util.ResourceNameCompletionFunc(f, "pod"),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(t, cmd, args))
			cmdutil.CheckErr(opts.Validate(t, cmd, args))
			cmdutil.CheckErr(opts.RunLogin(t, cmd, args, loginStore))
		},
	}
	return cmd
}

func (o *LoginOptions) Complete(_ *terminal.Terminal, _ *cobra.Command, _ []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *LoginOptions) Validate(_ *terminal.Terminal, _ *cobra.Command, _ []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *LoginOptions) RunLogin(t *terminal.Terminal, _ *cobra.Command, _ []string, loginStore LoginStore) error {
	// func (o *LoginOptions) RunLogin(cmd *cobra.Command, args []string) error {
	token, err := auth.Login(false)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = postLogin(*token, loginStore, t)
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
func postLogin(token string, loginStore LoginStore, t *terminal.Terminal) error {
	// TODO: hit GetMe and if fails create user
	client, err := brevapi.NewCommandClient()
	if err != nil {
		return err
	}
	_, err = client.GetMe()
	if err != nil {
		// TODO: if the error is not a network call create the account
		// _, err := client.CreateUser(creds.IDToken)
		_, err := loginStore.CreateUser(token)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		// TODO: create org if none
		orgs, err := loginStore.GetOrganizations()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		firstOrgName := "firstorg-hq"
		if len(orgs) == 0 {
			org, innererr := loginStore.CreateOrganization(store.CreateOrganizationRequest{
				// TODO: get the username from GetMe function
				Name: firstOrgName,
			})
			if innererr != nil {
				return breverrors.WrapAndTrace(innererr)
			}

			fmt.Println("Created your first org " + firstOrgName)

			path := files.GetActiveOrgsPath()
			innererr = files.OverwriteJSON(path, org)
			if innererr != nil {
				return breverrors.WrapAndTrace(innererr)
			}
		} else {
			// TODO: check that there's no active, but realistically, this should never happen.
			path := files.GetActiveOrgsPath()
			err = files.OverwriteJSON(path, orgs[0])
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
		}

		// SSH Keys
		brevapi.DisplayBrevLogo(t)
		t.Vprintf("\n")
		spinner := t.NewSpinner()
		spinner.Suffix = " fetching your public key"
		spinner.Start()
		err = brevapi.GetandDisplaySSHKeys(t)
		spinner.Stop()
		if err != nil {
			t.Vprintf(t.Red(err.Error()))
		}

		// TODO: check if user uses VSCode and intall extension for user
		hasVSCode := brevapi.PromptSelectInput(brevapi.PromptSelectContent{
			Label:    "Do you use VS Code?",
			ErrorMsg: "error",
			Items:    []string{"yes", "no"},
		})
		if hasVSCode == "yes" {
			// TODO: check if user uses VSCode and intall extension for user
			t.Vprintf(t.Yellow("Please install the following VS Code extension: ms-vscode-remote.remote-ssh\n"))
		}
	}
	return nil
}
