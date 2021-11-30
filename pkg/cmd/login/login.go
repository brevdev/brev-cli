// Package login is for the get command
package login

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	t.Print("")
	t.Print("Welcome to Brev 🤙 ")
	t.Print("")
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

	// // SSH Keys
	terminal.DisplayBrevLogo(t)
	t.Print("")
	terminal.DisplaySSHKeys(t, user.PublicKey)

	ide := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "What is your preferred IDE?",
		ErrorMsg: "error",
		Items:    []string{"VSCode", "JetBrains IDEs"},
	})
	if ide == "VSCode" {
		// TODO: check if user uses VSCode and intall extension for user
		t.Print("Run the following steps")
		t.Print("")
		t.Print("\t1) Install the following VSCode extension: " + t.Yellow("ms-vscode-remote.remote-ssh") + ".")
		t.Print("\t2) Run " + t.Green("brev up") + " and then open VSCode.")
		t.Print("\t3) Open the Command Palette and type in " + t.Yellow("Remote-SSH: Connect to Host...") + "to begin.")
		return nil
	}

	if ide == "JetBrains IDEs" {
		localOS := terminal.PromptSelectInput(terminal.PromptSelectContent{
			Label:    "Which operating system does your local computer have?",
			ErrorMsg: "error",
			Items:    []string{"Mac OS (Intel)", "Mac OS (M1 Chip)", "Linux"},
		})

		homeDirectory, err := os.UserHomeDir()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		var jetBrainsDirectory string
		var installScript string
		switch localOS {
		case "Mac OS (Intel)":
			jetBrainsDirectory = homeDirectory + `/Library/Application Support/JetBrains`
			installScript = `set -e; echo "Start installation..."; wget --show-progress -qO ./gateway.dmg "https://data.services.jetbrains.com/products/download?code=GW&platform=mac&type=eap,rc,release,beta"; hdiutil attach gateway.dmg; cp -R "/Volumes/JetBrains Gateway/JetBrains Gateway.app" /Applications; hdiutil unmount "/Volumes/JetBrains Gateway"; rm ./gateway.dmg; echo "JetBrains Gateway successfully installed"`
		case "Mac OS (M1 Chip)":
			jetBrainsDirectory = homeDirectory + `/Library/Application Support/JetBrains`
			installScript = `set -e; echo "Start installation..."; wget --show-progress -qO ./gateway.dmg "https://data.services.jetbrains.com/products/download?code=GW&platform=macM1&type=eap,rc,release,beta"; hdiutil attach gateway.dmg; cp -R "/Volumes/JetBrains Gateway/JetBrains Gateway.app" /Applications; hdiutil unmount "/Volumes/JetBrains Gateway"; rm ./gateway.dmg; echo "JetBrains Gateway successfully installed"`
		case "Linux":
			jetBrainsDirectory = homeDirectory + `/.local/share/JetBrains`
			installScript = `set -e; echo "Start installation..."; wget --show-progress -qO ./gateway.tar.gz "https://data.services.jetbrains.com/products/download?code=GW&platform=linux&type=eap,rc,release,beta"; GATEWAY_TEMP_DIR=$(mktemp -d); tar -C "$GATEWAY_TEMP_DIR" -xf gateway.tar.gz; rm ./gateway.tar.gz; "$GATEWAY_TEMP_DIR"/*/bin/gateway.sh; rm -r "$GATEWAY_TEMP_DIR"; echo "JetBrains Gateway was successfully installed";`
		}

		t.Print("Searching for JetBrains Gateway...")
		t.Print("")

		// Check if Gateway is already installed
		matches, err := filepath.Glob(jetBrainsDirectory + `/JetBrainsGateway*`)
		if len(matches) > 0 {
			t.Print(t.Yellow("You already have JetBrains Gateway installed!"))
			t.Print("Run " + t.Green("brev up") + " and then open Gateway to begin.")
			return nil
		}

		// Check if Toolbox is already installed
		_, err = os.Stat(jetBrainsDirectory + `/Toolbox`)
		if !os.IsNotExist(err) {
			// Check if Gateway is already installed in Toolbox
			_, err = os.Stat(jetBrainsDirectory + `/Toolbox/apps/JetBrainsGateway`)
			if !os.IsNotExist(err) {
				t.Print(t.Yellow("You already have JetBrains Gateway installed!"))
				t.Print("Run " + t.Green("brev up") + " and then open Gateway to begin.")
				return nil
			}
			t.Print(t.Yellow("You already have JetBrains Toolbox installed!"))
			t.Print("")
			t.Print("\t1) Install JetBrains Gateway from Toolbox.")
			t.Print("\t2) Run " + t.Green("brev up") + " and then open Gateway to begin.")
			return nil
		}

		// Install Gateway
		reader := bufio.NewReader(os.Stdin)
		t.Print("You will need to install JetBrains Gateway to use JetBrains IDEs.")
		t.Print(`Would you like to install JetBrains Gateway? [y/n]: `)
		text, _ := reader.ReadString('\n')
		if strings.Compare(text, "y") != 1 {
			// n
			t.Print("")
			t.Print(t.Yellow("Click here to install manually: "))
			t.Print("https://www.jetbrains.com/remote-development/gateway")
			t.Print("")
		} else {
			// y
			spinner := t.NewSpinner()
			spinner.Suffix = " Installing JetBrains Gateway..."
			spinner.Start()
			cmd := exec.Command("/bin/sh", "-c", installScript)
			_, err := cmd.Output()
			spinner.Stop()
			if err != nil {
				t.Errprint(err, "Error installing JetBrains Gateway")
				t.Print("")
				t.Print(t.Yellow("Click here to install manually: "))
				t.Print("https://www.jetbrains.com/remote-development/gateway")
				t.Print("")
			} else {
				t.Print("")
				t.Print("")
				t.Print(t.Yellow("JetBrains Gateway successfully installed!"))
				t.Print("")
				t.Print("Run " + t.Green("brev up") + " and then open Gateway to begin.")
				t.Print("")
			}
		}
		return nil
	}
	return nil
}

func makeFirstOrgName(user *brevapi.User) string {
	return fmt.Sprintf("%s-hq", user.Username)
}
