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
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type LoginOptions struct {
	Auth       Auth
	LoginStore LoginStore
}

type LoginStore interface {
	GetCurrentUser() (*entity.User, error)
	CreateUser(idToken string) (*entity.User, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	CreateOrganization(req store.CreateOrganizationRequest) (*entity.Organization, error)
}

type Auth interface {
	Login() (*auth.LoginTokens, error)
}

// loginStore must be a no prompt store
func NewCmdLogin(t *terminal.Terminal, loginStore LoginStore, auth Auth) *cobra.Command {
	opts := LoginOptions{
		Auth:       auth,
		LoginStore: loginStore,
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
	tokens, err := o.Auth.Login()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = o.LoginStore.GetCurrentUser()
	if err != nil {
		// TODO 1 check if 400/404
		err := CreateNewUser(o.LoginStore, tokens.IDToken, t)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func CreateNewUser(loginStore LoginStore, idToken string, t *terminal.Terminal) error {
	t.Print("\nWelcome to Brev 🤙\n")
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
	t.Print("")
	terminal.DisplaySSHKeys(t, user.PublicKey)

	t.Vprint("\n")

	// Check IDE requirements
	doneAddingKey := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "Finished adding your SSH key (above)?",
		ErrorMsg: "error",
		Items:    []string{"Yes", "No", "Skip"},
	})

	if doneAddingKey == "skip" || doneAddingKey == "no" {
		t.Vprint(t.Red("\nFeel free to proceed but you will not be able to pull or push your private repos. Run 'brev ssh-key' to do this step later\n"))
	}

	// Check IDE requirements
	ide := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "What is your preferred IDE?",
		ErrorMsg: "error",
		Items:    []string{"VSCode", "JetBrains IDEs"},
	})
	if ide == "VSCode" {
		// TODO: check if user uses VSCode and intall extension for user
		terminal.DisplayVSCodeInstructions(t)
		return nil
	}

	if ide == "JetBrains IDEs" {
		return CheckAndInstallGateway(t)
	}
	return nil
}

func CheckAndInstallGateway(t *terminal.Terminal) (err error) {
	localOS := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "Which operating system does your local computer have?",
		ErrorMsg: "error",
		Items:    []string{"Mac OS (Intel)", "Mac OS (M1 Chip)", "Linux"},
	})

	jetBrainsDirectory, installScript, err := CreateDownloadPathAndInstallScript(localOS)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Print("Searching for JetBrains Gateway...")
	t.Print("")

	// Check if Gateway is already installed
	gatewayInstalled := IsGatewayInstalled(jetBrainsDirectory)
	if gatewayInstalled {
		terminal.DisplayGatewayAlreadyInstalledInstructions(t)
		return nil
	}

	// Check if Toolbox is already installed
	toolboxInstalled, gatewayInToolboxInstalled := IsToolboxInstalled(jetBrainsDirectory)
	if gatewayInToolboxInstalled {
		terminal.DisplayGatewayAlreadyInstalledInstructions(t)
		return nil
	}

	if toolboxInstalled {
		terminal.DisplayToolboxInstalledInstructions(t)
		return nil
	}

	InstallGateway(installScript, t)
	return nil
}

func IsToolboxInstalled(jetBrainsBaseDirectory string) (toolboxInstalled bool, gatewayInToolboxInstalled bool) {
	_, err := os.Stat(jetBrainsBaseDirectory + `/Toolbox`)
	if !os.IsNotExist(err) {
		// Check if Gateway is already installed in Toolbox
		_, err = os.Stat(jetBrainsBaseDirectory + `/Toolbox/apps/JetBrainsGateway`)
		return true, !os.IsNotExist(err)
	}
	return false, false
}

func IsGatewayInstalled(jetBrainsBaseDirectory string) bool {
	matches, _ := filepath.Glob(jetBrainsBaseDirectory + `/JetBrainsGateway*`)
	return len(matches) > 0
}

func InstallGateway(installScript string, t *terminal.Terminal) {
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
		// #nosec
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
			t.Print("Run " + t.Green("brev jetbrains") + " and then open Gateway to begin.")
			t.Print("")
		}
	}
}

func CreateDownloadPathAndInstallScript(localOS string) (jetBrainsDirectory string, installScript string, err error) {
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return "", "", breverrors.WrapAndTrace(err)
	}
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
	return jetBrainsDirectory, installScript, nil
}

func makeFirstOrgName(user *entity.User) string {
	return fmt.Sprintf("%s-hq", user.Username)
}
