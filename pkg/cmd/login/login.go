// Package login is for the get command
package login

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type LoginOptions struct {
	auth       Auth
	loginStore LoginStore
}

type LoginStore interface {
	GetCurrentUser() (*brevapi.User, error)
	CreateUser(idToken string) (*brevapi.User, error)
	GetOrganizations() ([]brevapi.Organization, error)
	CreateOrganization(req store.CreateOrganizationRequest) (*brevapi.Organization, error)
}

type Auth interface {
	Login() (*auth.LoginTokens, error)
}

// loginStore must be a no prompt store
func NewCmdLogin(t *terminal.Terminal, loginStore LoginStore, auth Auth) *cobra.Command {
	opts := LoginOptions{
		auth:       auth,
		loginStore: loginStore,
	}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "login",
		DisableFlagsInUseLine: true,
		Short:                 "Log into brev",
		Long:                  "Log into brev",
		Example:               "brev login",
		Args:                  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(t, cmd, args))
			cmdutil.CheckErr(opts.RunLogin(t))
		},
	}
	return cmd
}

func (o *LoginOptions) Complete(_ *terminal.Terminal, _ *cobra.Command, _ []string) error {
	return nil
}

func (o LoginOptions) RunLogin(t *terminal.Terminal) error {
	// func (o *LoginOptions) RunLogin(cmd *cobra.Command, args []string) error {
	tokens, err := o.auth.Login()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = o.loginStore.GetCurrentUser()
	if err != nil {
		// TODO 1 check if 400/404
		err := CreateNewUser(o.loginStore, tokens.IDToken, t)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func CreateNewUser(loginStore LoginStore, idToken string, t *terminal.Terminal) error {
	user, err := loginStore.CreateUser(idToken)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	orgs, err := loginStore.GetOrganizations()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if len(orgs) == 0 {
		orgName := makeFirstOrgName(user)
		_, err := loginStore.CreateOrganization(store.CreateOrganizationRequest{
			Name: orgName,
		})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	// SSH Keys
	terminal.DisplayBrevLogo(t)
	t.Vprintf("\n")
	terminal.DisplaySSHKeys(t, user.PublicKey)

	hasVSCode := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "Do you use VS Code?",
		ErrorMsg: "error",
		Items:    []string{"yes", "no"},
	})
	if hasVSCode == "yes" {
		// TODO: check if user uses VSCode and intall extension for user
		t.Vprintf(t.Yellow("Please install the following VS Code extension: ms-vscode-remote.remote-ssh\n"))
	}

	return nil
}

func makeFirstOrgName(user *brevapi.User) string {
	return fmt.Sprintf("%s-hq", user.Username)
}
