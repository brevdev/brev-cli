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
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/importideconfig"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/server"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/util"
	"github.com/brevdev/brev-cli/pkg/vpn"
	"github.com/fatih/color"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"golang.org/x/text/encoding/charmap"
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
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	CreateOrganization(req store.CreateOrganizationRequest) (*entity.Organization, error)
	GetServerSockFile() string
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
}

type Auth interface {
	Login() (*auth.LoginTokens, error)
	LoginWithToken(token string) error
}

// loginStore must be a no prompt store
func NewCmdLogin(t *terminal.Terminal, loginStore LoginStore, auth Auth) *cobra.Command {
	opts := LoginOptions{
		Auth:       auth,
		LoginStore: loginStore,
	}

	var loginToken string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "login",
		DisableFlagsInUseLine: true,
		Short:                 "Log into brev",
		Long:                  "Log into brev",
		Example:               "brev login",
		Args:                  cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := opts.RunLogin(t, loginToken)
			if err != nil {
				err2 := RunTasksForUser(t)
				if err2 != nil {
					err = multierror.Append(err, err2)
				}
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&loginToken, "token", "", "", "token provided to auto login")
	return cmd
}

func (o LoginOptions) checkIfInWorkspace() error {
	workspaceID, err := o.LoginStore.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		fmt.Println("can not login to workspace")
		return breverrors.NewValidationError("can not login to workspace")
	}

	return nil
}

func (o LoginOptions) loginAndGetOrCreateUser(loginToken string) (*entity.User, error) {
	if loginToken != "" {
		err := o.Auth.LoginWithToken(loginToken)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	} else {
		tokens, err := o.Auth.Login()
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}

		_, err = CreateNewUser(o.LoginStore, tokens.IDToken)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}

	user, err := o.LoginStore.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return user, nil
}

func (o LoginOptions) getOrCreateOrg(username string) (*entity.Organization, error) {
	org, err := o.LoginStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if org == nil {
		newOrgName := makeFirstOrgName(username)
		fmt.Printf("Creating your first org %s ...\n", newOrgName)
		org, err = o.LoginStore.CreateOrganization(store.CreateOrganizationRequest{
			Name: newOrgName,
		})
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		fmt.Println("done!")
	}
	return org, nil
}

func (o LoginOptions) RunLogin(t *terminal.Terminal, loginToken string) error {
	err := o.checkIfInWorkspace()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	caretType := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Print("\n")
	fmt.Println("  ", caretType("▸"), "    Starting Login")
	fmt.Print("\n")

	user, err := o.loginAndGetOrCreateUser(loginToken)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	org, err := o.getOrCreateOrg(user.Username)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = o.handleOnboaring(user, t)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = o.showBreadCrumbs(t, org, user)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if featureflag.IsAdmin(user.GlobalUserType) {
		err1 := o.HandleLoginAdmin()
		if err1 != nil {
			return breverrors.WrapAndTrace(err1)
		}
	}

	err = importideconfig.RunImportIDEConfig(t, o.LoginStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (o LoginOptions) handleOnboaring(user *entity.User, t *terminal.Terminal) error {
	// figure out if we should onboard the user
	currentOnboardingStatus, err := user.GetOnboardingStatus()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	newOnboardingStatus := make(map[string]interface{})
	// todo make this one api call
	if !currentOnboardingStatus.SSH {
		err = OnboardUserWithSSHKeys(t, user, o.LoginStore, true)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		newOnboardingStatus["SSH"] = true
	}

	var ide string
	if currentOnboardingStatus.Editor == "" {
		// Check IDE requirements
		ide = terminal.PromptSelectInput(terminal.PromptSelectContent{
			Label:    "What is your preferred IDE?",
			ErrorMsg: "error",
			Items:    []string{"VSCode", "JetBrains IDEs", "Vim", "Emacs", "Atom", "Other"},
		})
		if ide == "Other" {
			ide = terminal.PromptGetInput(terminal.PromptContent{
				Label:    "Please enter your preferred IDE:",
				ErrorMsg: "Error: must enter a value",
			})
		}
		newOnboardingStatus["editor"] = ide
	} else {
		ide = currentOnboardingStatus.Editor
	}
	_, err = OnboardUserWithEditors(t, o.LoginStore, ide)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if !currentOnboardingStatus.UsedCLI {
		// by getting this far, we know they have set up the cli
		newOnboardingStatus["usedCLI"] = true
	}

	user, err = o.LoginStore.UpdateUser(user.ID, &entity.UpdateUser{
		// username, name, and email are required fields, but we only care about onboarding status
		Username:         user.Username,
		Name:             user.Name,
		Email:            user.Email,
		OnboardingStatus: util.MapAppend(user.OnboardingStatus, newOnboardingStatus),
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func getOtherOrg(org *entity.Organization, orgs []entity.Organization) *entity.Organization {
	for _, o := range orgs {
		if o.ID != org.ID {
			return &o
		}
	}
	return nil
}

func (o LoginOptions) HandleLoginAdmin() error {
	fmt.Println("\nsetting up admin...")
	sock := o.LoginStore.GetServerSockFile()
	c, err := server.NewClient(sock)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = c.ConfigureVPN()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func RunTasksForUser(t *terminal.Terminal) error {
	brevBin, err := os.Executable()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	cmd := exec.Command(brevBin, "run-tasks", "-d") // #nosec G204
	err = cmd.Run()
	if err != nil {
		fmt.Print("\n")
		t.Vprint(t.Red("tasks failed to run ") + t.Yellow("brev run-tasks -d") + t.Red(" in your terminal."))
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// returns if the user is indeed new
func CreateNewUser(loginStore LoginStore, idToken string) (bool, error) {
	_, err := loginStore.CreateUser(idToken)
	if err != nil {
		if !strings.Contains(err.Error(), "duplicate username") {
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

	fmt.Print("\n")

	t.Vprint(t.Green("Add the key to your git repo provider"))
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

func (o LoginOptions) showBreadCrumbs(t *terminal.Terminal, org *entity.Organization, user *entity.User) error {
	orgs, err := o.LoginStore.GetOrganizations(nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	allWorkspaces, err := o.LoginStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	userWorkspaces := []entity.Workspace{}
	for _, w := range allWorkspaces {
		if w.CreatedByUserID == user.ID {
			userWorkspaces = append(userWorkspaces, w)
		}
	}

	t.Vprintf(t.Green("current organization: ") + t.Yellow(fmt.Sprintf("%s\n", org.Name)))
	if len(orgs) > 1 {
		otherOrg := getOtherOrg(org, orgs)
		t.Vprintf(t.Green("switch organizations:\n"))
		t.Vprintf(t.Yellow("\tbrev set %s\n", otherOrg.Name))
	}

	if len(allWorkspaces) == 0 {
		t.Vprintf(t.Green("create a workspace:\n"))
		t.Vprintf(t.Yellow("\tbrev start https://github.com/brevdev/hello-react\n"))
	}
	if len(userWorkspaces) == 0 && len(allWorkspaces) > 1 {
		t.Vprintf(t.Green("list teammates workspaces:\n"))
		t.Vprintf(t.Yellow("\tbrev ls --all\n"))

		t.Vprintf(t.Green("join a teammate's workspace:\n"))
		t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev start %s\n", allWorkspaces[0].Name)))
	}
	if len(userWorkspaces) > 0 {
		t.Vprintf(t.Green("list your workspaces:\n"))
		t.Vprintf(t.Yellow("\tbrev ls\n"))
	}

	return nil
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

func makeFirstOrgName(username string) string {
	return fmt.Sprintf("%s-hq", username)
}
