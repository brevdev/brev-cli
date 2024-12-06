// Package login is for the get command
package login

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/hello"

	"github.com/brevdev/brev-cli/pkg/cmd/importideconfig"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/util"
	"github.com/fatih/color"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
)

type LoginOptions struct {
	Auth       Auth
	LoginStore LoginStore
}

type LoginStore interface {
	auth.AuthStore
	GetCurrentUser() (*entity.User, error)
	CreateUser(idToken string) (*entity.User, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	CreateOrganization(req store.CreateOrganizationRequest) (*entity.Organization, error)
	GetServerSockFile() string
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
	hello.HelloStore
	importideconfig.ImportIDEConfigStore
	UserHomeDir() (string, error)
}

type Auth interface {
	Login(skipBrowser bool) (*auth.LoginTokens, error)
	LoginWithToken(token string) error
}

// loginStore must be a no prompt store
func NewCmdLogin(t *terminal.Terminal, loginStore LoginStore, auth Auth) *cobra.Command {
	opts := LoginOptions{
		Auth:       auth,
		LoginStore: loginStore,
	}

	var loginToken string
	var skipBrowser bool
	var emailFlag string
	var authProviderFlag string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "login",
		DisableFlagsInUseLine: true,
		Short:                 "Log into brev",
		Long:                  "Log into brev",
		Example:               "brev login",
		PostRunE: func(cmd *cobra.Command, args []string) error {
			shouldWe := hello.ShouldWeRunOnboarding(loginStore)
			if shouldWe {
				user, err := loginStore.GetCurrentUser()
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
				err = hello.CanWeOnboard(t, user, loginStore)
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
			}
			return nil
		},
		Args: cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := opts.RunLogin(t, loginToken, skipBrowser, emailFlag, authProviderFlag)
			if err != nil {
				// if err is ImportIDEConfigError, log err with sentry but continue
				if _, ok := err.(*importideconfig.ImportIDEConfigError); !ok {
					return err
				}
				// todo alert sentry
				err2 := RunTasksForUser(t)
				if err2 != nil {
					err = multierror.Append(err, err2)
				}
				return err //nolint:wrapcheck // we want to return the error from the login
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&loginToken, "token", "", "", "token provided to auto login")
	cmd.Flags().BoolVar(&skipBrowser, "skip-browser", false, "print url instead of auto opening browser")
	cmd.Flags().StringVar(&emailFlag, "email", "", "email to use for authentication")
	cmd.Flags().StringVar(&authProviderFlag, "auth", "", "authentication provider to use (nvidia or legacy, default is legacy)")
	return cmd
}

func (o LoginOptions) loginAndGetOrCreateUser(loginToken string, skipBrowser bool) (*entity.User, error) {
	if loginToken != "" {
		err := o.Auth.LoginWithToken(loginToken)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	} else {
		tokens, err := o.Auth.Login(skipBrowser)
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

func (o LoginOptions) RunLogin(t *terminal.Terminal, loginToken string, skipBrowser bool, emailFlag string, authProviderFlag string) error {
	tokens, _ := o.LoginStore.GetAuthTokens()

	if authProviderFlag != "" && authProviderFlag != "nvidia" && authProviderFlag != "legacy" {
		return breverrors.NewValidationError("auth provider must be nvidia or legacy")
	}

	authenticator := auth.StandardLogin(authProviderFlag, emailFlag, tokens)

	if emailFlag != "" && authenticator.GetCredentialProvider() != auth.CredentialProviderKAS {
		return breverrors.NewValidationError("email flag can only be used with nvidia auth provider")
	}

	o.Auth = auth.NewAuth(o.LoginStore, authenticator)

	caretType := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Print("\n")
	fmt.Println("  ", caretType("▸"), "    Starting Login")
	fmt.Print("\n")

	user, err := o.loginAndGetOrCreateUser(loginToken, skipBrowser)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	org, err := o.getOrCreateOrg(user.Username)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = o.handleOnboarding(user, t)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = o.showBreadCrumbs(t, org, user)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (o LoginOptions) handleOnboarding(user *entity.User, t *terminal.Terminal) error {
	// figure out if we should onboard the user
	currentOnboardingStatus, err := user.GetOnboardingData()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	newOnboardingStatus := make(map[string]interface{})
	// todo make this one api call
	// if !currentOnboardingStatus.SSH {
	// 	err = OnboardUserWithSSHKeys(t, user, o.LoginStore, true)
	// 	if err != nil {
	// 		return breverrors.WrapAndTrace(err)
	// 	}
	// 	newOnboardingStatus["SSH"] = true
	// }

	var ide string
	if currentOnboardingStatus.Editor == "" {
		// Check IDE requirements
		ide = terminal.PromptSelectInput(terminal.PromptSelectContent{
			Label:    "What is your preferred IDE?",
			ErrorMsg: "Error: must choose a preferred IDE",
			Items:    []string{"VSCode", "Vim", "Emacs"},
		})
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
		Username:       user.Username,
		Name:           user.Name,
		Email:          user.Email,
		OnboardingData: util.MapAppend(user.OnboardingData, newOnboardingStatus),
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
		if !(strings.Contains(err.Error(), "duplicate username") || strings.Contains(err.Error(), "duplicate external auth id") || strings.Contains(err.Error(), "identity already exists")) {
			// This is a real error
			return false, breverrors.WrapAndTrace(err)
		}
		return false, nil
	}

	return true, nil
}

// SSH Keys

// t.Eprintf(t.Yellow("\n\tClick here for Gitlab: https://gitlab.com/-/profile/keys\n"))

// t.Vprint(t.Red("\nYou must add your SSH key to pull and push from your repos. "))

func OnboardUserWithEditors(t *terminal.Terminal, _ LoginStore, ide string) (string, error) {
	if ide == "VSCode" {
		_ = 0
	} else {
		t.Print("To use " + ide + " for your instance. Use the following command to remote into your machine")
		t.Print(t.Green("Brev Shell"))
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
		t.Vprintf(t.Green("create an instance:\n"))
	}
	if len(userWorkspaces) == 0 && len(allWorkspaces) > 1 {
		t.Vprintf(t.Green("list teammates instances:\n"))
		t.Vprintf(t.Yellow("\tbrev ls --all\n"))

		t.Vprintf(t.Green("clone a teammate's instance:\n"))
		t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev start %s\n", allWorkspaces[0].Name)))
	}
	if len(userWorkspaces) > 0 {
		t.Vprintf(t.Green("list your instances:\n"))
		t.Vprintf(t.Yellow("\tbrev ls\n"))
	}

	return nil
}

// Check if Gateway is already installed

// Check if Toolbox is already installed

// Check if Gateway is already installed in Toolbox

// n

// y

// #nosec

func makeFirstOrgName(username string) string {
	return fmt.Sprintf("%s-hq", username)
}
