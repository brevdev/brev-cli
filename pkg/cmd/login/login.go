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
	GetOrganizations(options *store.GetOrganizationsOptions) ([]brevapi.Organization, error)
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
	t.Print("Creating your user...")
	user, err := loginStore.CreateUser(idToken)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	orgs, err := loginStore.GetOrganizations(nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if len(orgs) == 0 {
		orgName := makeFirstOrgName(user)
		t.Printf("Creating your first org %s", orgName)
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

	ide := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "What is your preferred IDE?",
		ErrorMsg: "error",
		Items:    []string{"VSCode", "JetBrains"},
	})
	if ide == "VSCode" {
		// TODO: check if user uses VSCode and intall extension for user
		t.Vprintf(t.Yellow("Please install the following VS Code extension: ms-vscode-remote.remote-ssh\n"))
		return nil
	}

	// which operating system

	// linux, mac m1, mac. In each do the path of where it's installed.
	// macOS: ~/Library/Application Support/JetBrains
	// Linux: ~/.local/share/JetBrains
	// home directory involved
	// os.HomeDirectory
	// check actual jetbrains gateway
	// filepath.Glob(home directory, middle, /JetBrainsGateway*)
	// f (to check print all the strings it returns)
	// return list of strings.. hmmm. if len(existingGateway) > 0.. f just print it. list of folder names that start with jetbrainsgateway*
	// check toolbox
	// os.Stat(/Toolbox)
	// check in toolbox apps
	// os.Stat(/Toolbox/apps/JetBrainsGateway)
	// run exec.Command("/bin/sh", "-c", installScript)

	// `set -e; echo "Start installation..."; wget --show-progress -qO ./gateway.dmg "https://data.services.jetbrains.com/products/download?code=GW&platform=mac&type=eap,rc,release,beta"; hdiutil attach gateway.dmg; cp -R "/Volumes/JetBrains Gateway/JetBrains Gateway.app" /Applications; hdiutil unmount "/Volumes/JetBrains Gateway"; rm ./gateway.dmg; echo "JetBrains Gateway successfully installed"`
	// `set -e; echo "Start installation..."; wget --show-progress -qO ./gateway.dmg "https://data.services.jetbrains.com/products/download?code=GW&platform=macM1&type=eap,rc,release,beta"; hdiutil attach gateway.dmg; cp -R "/Volumes/JetBrains Gateway/JetBrains Gateway.app" /Applications; hdiutil unmount "/Volumes/JetBrains Gateway"; rm ./gateway.dmg; echo "JetBrains Gateway successfully installed"`
	// `set -e; echo "Start installation..."; wget --show-progress -qO ./gateway.tar.gz "https://data.services.jetbrains.com/products/download?code=GW&platform=linux&type=eap,rc,release,beta"; GATEWAY_TEMP_DIR=$(mktemp -d); tar -C "$GATEWAY_TEMP_DIR" -xf gateway.tar.gz; rm ./toolbox.tar.gz; "$GATEWAY_TEMP_DIR"/*/bin/gateway.sh; rm -r "$GATEWAY_TEMP_DIR"; echo "JetBrains Gateway was successfully installed";`

	// t.Vprintf("\nYou already have JetBrains Gateway installed through JetBrains Toolbox! Run " + t.Green("brev up") + " and then open Gateway to begin!. \n")
	// t.Vprintf("\nError installing JetBrains Gateway. Click here to install manually: \n")
	// t.Vprintf("\n\thttps://www.jetbrains.com/remote-development/gateway \n")

	// after this, was going to add loading bar here during this part
	// and then change configs which are located at (see installing on linux and go to the right directory and look through the install script before deleting)
	return nil
}

func makeFirstOrgName(user *brevapi.User) string {
	return fmt.Sprintf("%s-hq", user.Username)
}
