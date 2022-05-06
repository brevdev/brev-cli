// Package login is for the get command
package login

import (
	"bufio"
	"errors"
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
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
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
			RunTasksForUser(t)
		},
	}
	return cmd
}

func RunTasksForUser(t *terminal.Terminal) {
	cmd := exec.Command("brev", "run-tasks", "-d") // #nosec G204
	err := cmd.Run()
	if err != nil {
		// tell user to run brev run-tasks
		t.Vprint(t.Red("\nPlease run ") + t.Yellow("brev run-tasks -d") + t.Red(" in your terminal."))
	}
}

func (o *LoginOptions) Complete(_ *terminal.Terminal, _ *cobra.Command, _ []string) error {
	return nil
}

func fetchThingsForTheNextFunction(o LoginOptions, t *terminal.Terminal) (*entity.User, bool, []entity.Organization, error) {
	workspaceID, err := o.LoginStore.GetCurrentWorkspaceID()
	if err != nil {
		return nil, false, nil, breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		fmt.Println("can not login to workspace")
		return nil, false, nil, errors.New("can not login to workspace")
	}

	tokens, err := o.Auth.Login()
	if err != nil {
		return nil, false, nil, breverrors.WrapAndTrace(err)
	}

	isUserCreated, err := CreateNewUser(o.LoginStore, tokens.IDToken, t)
	if err != nil {
		return nil, false, nil, breverrors.WrapAndTrace(err)
	}

	user, err := o.LoginStore.GetCurrentUser()
	if err != nil {
		return nil, isUserCreated, nil, breverrors.WrapAndTrace(err)
	}

	orgs, err := o.LoginStore.GetOrganizations(nil)
	if err != nil {
		return user, isUserCreated, nil, breverrors.WrapAndTrace(err)
	}

	return user, isUserCreated, orgs, nil
}

func mapAppend(m map[string]interface{}, n ...map[string]interface{}) map[string]interface{} {
	if m == nil { // we may get nil maps from legacy users not having user.OnboardingStatus set
		m = make(map[string]interface{})
	}
	for _, item := range n {
		for key, value := range item {
			m[key] = value
		}
	}
	return m
}

func (o LoginOptions) RunLogin(t *terminal.Terminal) error {
	user, isUserCreated, orgs, err := fetchThingsForTheNextFunction(o, t)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	// Create a users first org
	if len(orgs) == 0 {
		orgName := makeFirstOrgName(user)
		t.Printf("Creating your first org %s ... ", orgName)
		_, err2 := o.LoginStore.CreateOrganization(store.CreateOrganizationRequest{
			Name: orgName,
		})
		if err2 != nil {
			return breverrors.WrapAndTrace(err2)
		}
		t.Print("done!")
	}

	// figure out if we should onboard the user
	currentOnboardingStatus, err := user.GetOnboardingStatus()
	if err != nil {
		// TODO: should we just proceed with something here?
		return breverrors.WrapAndTrace(err)
	}
	newOnboardingStatus := make(map[string]interface{})

	// todo make this one api call
	if !currentOnboardingStatus.SSH {
		_ = OnboardUserWithSSHKeys(t, user, o.LoginStore, true)
		newOnboardingStatus["SSH"] = true
	}

	var ide string
	if currentOnboardingStatus.Editor == "" {
		// Check IDE requirements
		ide = terminal.PromptSelectInput(terminal.PromptSelectContent{
			Label:    "What is your preferred IDE?",
			ErrorMsg: "error",
			Items:    []string{"VSCode", "JetBrains IDEs"},
		})
		newOnboardingStatus["editor"] = ide
	} else {
		ide = currentOnboardingStatus.Editor
	}
	OnboardUserWithEditors(t, o.LoginStore, ide)

	if !currentOnboardingStatus.UsedCLI {
		// by getting this far, we know they have set up the cli
		newOnboardingStatus["usedCLI"] = true
	}

	user, err = o.LoginStore.UpdateUser(user.ID, &entity.UpdateUser{
		// username, name, and email are required fields, but we only care about onboarding status
		Username:         user.Username,
		Name:             user.Name,
		Email:            user.Email,
		OnboardingStatus: mapAppend(user.OnboardingStatus, newOnboardingStatus),
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	FinalizeOnboarding(t, isUserCreated)

	if featureflag.IsAdmin(user.GlobalUserType) {
		s := t.NewSpinner()
		s.Suffix = " setting up service mesh bcs you're an ADMIN 🤩"
		s.Start()

		sock := o.LoginStore.GetServerSockFile()
		c, err := server.NewClient(sock)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = c.ConfigureVPN()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		s.Stop()
	}
	return nil
}

// returns if the user is indeed new
func CreateNewUser(loginStore LoginStore, idToken string, t *terminal.Terminal) (bool, error) {
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

func OnboardUserWithSSHKeys(t *terminal.Terminal, user *entity.User, _ LoginStore, firstLoop bool) error {
	if firstLoop {
		t.Vprint(t.Yellow("\n\nYou must add your SSH key to pull and push from your repos.\n"))
	} else {
		t.Vprint(t.Red("\n\nYou must add your SSH key to pull and push from your repos.\n"))
	}
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

	isFin := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "Did you finish adding your SSH key?",
		Items:    []string{"Yes", "No"},
		ErrorMsg: "error",
	})
	if isFin == "Yes" {
		return nil
	} else {
		// t.Vprint(t.Red("\nYou must add your SSH key to pull and push from your repos. "))
		err := OnboardUserWithSSHKeys(t, user, nil, false)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	return nil
}

func OnboardUserWithEditors(t *terminal.Terminal, loginStore LoginStore, ide string) (string, error) {
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
		return ide, CheckAndInstallGateway(t, loginStore)
	}

	return ide, nil
}

func FinalizeOnboarding(t *terminal.Terminal, isUserCreated bool) {
	terminal.DisplayBrevLogo(t)
	t.Vprint("\n")
	if isUserCreated {
		t.Vprintf(t.Green("\nCreate your first workspace! Try this command:"))
		t.Vprintf(t.Yellow("\n\t$brev start https://github.com/brevdev/hello-react"))
		t.Vprint("\n")
	}
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
