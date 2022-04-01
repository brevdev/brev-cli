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
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/server"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/vpn"
	"github.com/spf13/cobra"
	"golang.org/x/text/encoding/charmap"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type LoginOptions struct {
	Auth       Auth
	LoginStore LoginStore
}

type LoginStore interface {
	vpn.ServiceMeshStore
	GetCurrentUser() (*entity.User, error)
	CreateUser(idToken string) (*entity.User, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	CreateOrganization(req store.CreateOrganizationRequest) (*entity.Organization, error)
	GetServerSockFile() string
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
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
	workspaceID, err := o.LoginStore.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		fmt.Println("can not login to workspace")
		return nil
	}

	tokens, err := o.Auth.Login()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	isUserCreated, err := CreateNewUser(o.LoginStore, tokens.IDToken, t)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	user, err := o.LoginStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	orgs, err := o.LoginStore.GetOrganizations(nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// figure out if we should onboard the user
	if isUserCreated {
		t.Print("User created!")
		if len(orgs) == 0 {
			orgName := makeFirstOrgName(user)
			t.Printf("Creating your first org %s ... ", orgName)
			_, err := o.LoginStore.CreateOrganization(store.CreateOrganizationRequest{
				Name: orgName,
			})
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			t.Print("done!")
		}
		_ = onboardUserWithSSHKeys(t, user, o.LoginStore)
		_ = onboardUserWithEditors(t, o.LoginStore)
		finalizeOnboarding(t)
	} else {
		if len(orgs) == 1 && orgs[0].Name!=makeFirstOrgName(user) {

			// if there are no workspaces in this org, probably onboard them
			// NOTE: if we let people invite into workspaces, this needs to be done
			workspaces, err := o.LoginStore.GetWorkspaces(orgs[0].ID, &store.GetWorkspacesOptions{UserID: user.ID})
			if err == nil && len(workspaces)==0 {
				// The SSH key is done in the front end when acct is created
				_ = onboardUserWithEditors(t, o.LoginStore)
				finalizeOnboarding(t)
			}
		}
	}

	if featureflag.IsAdmin(user.GlobalUserType) {
		sock := o.LoginStore.GetServerSockFile()
		c, err := server.NewClient(sock)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = c.ConfigureVPN()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

// returns if the user is indeed new
func CreateNewUser(loginStore LoginStore, idToken string, t *terminal.Terminal) (bool, error) { //nolint:funlen // its 2 lines too long but not worth refac yet
	t.Print("\nWelcome to Brev.dev 🤙\n")
	_, err := loginStore.CreateUser(idToken)
	if err != nil {
		if !strings.Contains(err.Error(), "400 Bad Request") {
			// This is a real error
			return false, breverrors.WrapAndTrace(err)
		}
		return false, nil
	}

	return true, nil
}

func onboardUserWithSSHKeys(t *terminal.Terminal, user *entity.User, loginStore LoginStore) error {
	// SSH Keys
	_ = terminal.PromptGetInput(terminal.PromptContent{
		Label:      "🔒 Click enter to get your secure SSH key:",
		ErrorMsg:   "error",
		AllowEmpty: true,
	})

	t.Vprintf("\n" + user.PublicKey + "\n\n")

	_ = terminal.PromptGetInput(terminal.PromptContent{
		Label:      "Copy your public key 👆 then hit enter",
		ErrorMsg:   "error",
		AllowEmpty: true,
	})


	t.Vprint(t.Green("\nAdd the key to your git repo provider"))
	t.Vprint(t.Green("\tClick here if you use Github 👉 https://github.com/settings/ssh/new\n\n"))
	// t.Eprintf(t.Yellow("\n\tClick here for Gitlab: https://gitlab.com/-/profile/keys\n"))
		
	_ = terminal.PromptGetInput(terminal.PromptContent{
		Label:      "Hit enter when finished",
		// Label:      "Hit enter when finished:",
		ErrorMsg:   "error",
		AllowEmpty: true,
	})

	return nil
}
func onboardUserWithEditors(t *terminal.Terminal, loginStore LoginStore) error {

	// Check IDE requirements
	ide := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "What is your preferred IDE?",
		ErrorMsg: "error",
		Items:    []string{"VSCode", "JetBrains IDEs"},
	})
	if ide == "VSCode" {
		// Check if user uses VSCode and intall extension for user
		isInstalled, err := isVSCodeExtensionInstalled("ms-vscode-remote.remote-ssh")
		if err != nil {
			t.Print(t.Red("Couldn't install the necessary VSCode extension automatically."))
			t.Print("\tPlease install the following VSCode extension: " + t.Yellow("ms-vscode-remote.remote-ssh") + ".\n")
			_ = terminal.PromptGetInput(terminal.PromptContent{
				Label:      "Hit enter when finished:",
				ErrorMsg:   "error",
				AllowEmpty: true,
			})
		}

		// If we couldn't check for the extension being installed, they likely don't have code in path and this step should be skipped
		if !isInstalled && err == nil {
			// attempt to install the extension
			cmd := exec.Command("code", "--install-extension", "ms-vscode-remote.remote-ssh", "--force") // #nosec G204
			_ = cmd.Run()

			// verify installation
			isInstalled, err := isVSCodeExtensionInstalled("ms-vscode-remote.remote-ssh")

			// tell the user to install manually if still not installed
			if !isInstalled || err != nil {
				t.Print(t.Red("Couldn't install the necessary VSCode extension automatically."))
				t.Print("\tPlease install the following VSCode extension: " + t.Yellow("ms-vscode-remote.remote-ssh") + ".\n")
				_ = terminal.PromptGetInput(terminal.PromptContent{
					Label:      "Hit enter when finished:",
					ErrorMsg:   "error",
					AllowEmpty: true,
				})
			}
		}
	}

	if ide == "JetBrains IDEs" {
		return CheckAndInstallGateway(t, loginStore)
	}

	return nil
}

func finalizeOnboarding(t *terminal.Terminal) {
	terminal.DisplayBrevLogo(t)
	t.Vprint("\n")
}

func isVSCodeExtensionInstalled(extensionID string) (bool, error) {
	cmdddd := exec.Command("code", "--list-extensions") // #nosec G204
	in, err := cmdddd.Output()
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}

	d := charmap.CodePage850.NewDecoder()
	out, err := d.Bytes(in)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	return strings.Contains(string(out), extensionID), nil
}

func CheckAndInstallGateway(t *terminal.Terminal, store LoginStore) error {
	localOS := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "Which operating system does your local computer have?",
		ErrorMsg: "error",
		Items:    []string{"Mac OS (Intel)", "Mac OS (M1 Chip)", "Linux"},
	})

	home, err := store.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	jetBrainsDirectory, installScript, err := CreateDownloadPathAndInstallScript(localOS, home)
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

func CreateDownloadPathAndInstallScript(localOS string, homeDirectory string) (jetBrainsDirectory string, installScript string, err error) {
	switch localOS {
	case "Mac OS (Intel)":
		jetBrainsDirectory = homeDirectory + `/Library/Application Support/JetBrains`
		installScript = `set -e; echo "Start installation..."; curl "https://data.services.jetbrains.com/products/download?code=GW&platform=macM1&type=eap,rc,release,beta" --output ./gateway.dmg -L; hdiutil attach gateway.dmg; cp -R "/Volumes/JetBrains Gateway/JetBrains Gateway.app" /Applications; hdiutil unmount "/Volumes/JetBrains Gateway"; rm ./gateway.dmg; echo "JetBrains Gateway successfully installed"`
	case "Mac OS (M1 Chip)":
		jetBrainsDirectory = homeDirectory + `/Library/Application Support/JetBrains`
		installScript = `set -e; echo "Start installation..."; curl "https://data.services.jetbrains.com/products/download?code=GW&platform=macM1&type=eap,rc,release,beta" --output ./gateway.dmg -L; hdiutil attach gateway.dmg; cp -R "/Volumes/JetBrains Gateway/JetBrains Gateway.app" /Applications; hdiutil unmount "/Volumes/JetBrains Gateway"; rm ./gateway.dmg; echo "JetBrains Gateway successfully installed"`
	case "Linux":
		jetBrainsDirectory = homeDirectory + `/.local/share/JetBrains`
		installScript = `set -e; echo "Start installation..."; wget --show-progress -qO ./gateway.tar.gz "https://data.services.jetbrains.com/products/download?code=GW&platform=linux&type=eap,rc,release,beta"; GATEWAY_TEMP_DIR=$(mktemp -d); tar -C "$GATEWAY_TEMP_DIR" -xf gateway.tar.gz; rm ./gateway.tar.gz; "$GATEWAY_TEMP_DIR"/*/bin/gateway.sh; rm -r "$GATEWAY_TEMP_DIR"; echo "JetBrains Gateway was successfully installed";`
	}
	return jetBrainsDirectory, installScript, nil
}

func makeFirstOrgName(user *entity.User) string {
	return fmt.Sprintf("%s-hq", user.Username)
}
