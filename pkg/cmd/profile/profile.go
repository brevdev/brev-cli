package profile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/start"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/fatih/color"
	"github.com/pkg/browser"

	"github.com/spf13/cobra"
)

var (
	startLong    = "Make changes to your profile"
	startExample = "brev profile --set-personal-config <git_url>"
)

type ProfileStore interface {
	completions.CompletionStore
	GetCurrentUser() (*entity.User, error)
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
}

func NewCmdProfile(t *terminal.Terminal, loginProfileStore ProfileStore, noLoginProfileStore ProfileStore) *cobra.Command {
	var personalSettingsRepo string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "profile",
		DisableFlagsInUseLine: true,
		Short:                 "Personal profile commands",
		Long:                  startLong,
		Example:               startExample,
		// Args:                  cmderrors.TransformToValidationError(cobra.NoArgs),
		ValidArgsFunction: completions.GetAllWorkspaceNameCompletionHandler(noLoginProfileStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := profile(personalSettingsRepo, t, loginProfileStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&personalSettingsRepo, "set-personal-config", "p", "", "set your personal config repo")

	return cmd
}

func goToProfileInConsole() {
	url := "https://console.brev.dev/profile"
	caretType := color.New(color.FgGreen, color.Bold).SprintFunc()
	enterType := color.New(color.FgGreen, color.Bold).SprintFunc()
	urlType := color.New(color.FgWhite, color.Bold).SprintFunc()
	// fmt.Println("\n" + url + "\n")
	_ = terminal.PromptGetInput(terminal.PromptContent{
		Label:      "  " + caretType("▸") + "    Press " + enterType("Enter") + " to edit your profile in browser",
		ErrorMsg:   "error",
		AllowEmpty: true,
	})

	fmt.Print("\n")

	err := browser.OpenURL(url)
	if err != nil {
		fmt.Println("Error opening browser. Please copy", urlType(url), "and paste it in your browser.")
	}
}

func profile(personalSettingsRepo string, t *terminal.Terminal, profileStore ProfileStore) error {
	user, err := profileStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if len(personalSettingsRepo) == 0 {
		goToProfileInConsole()
		return nil
	}

	isURL := false
	if strings.Contains(personalSettingsRepo, "https://") || strings.Contains(personalSettingsRepo, "git@") {
		isURL = true
	}

	if !isURL {
		err = errors.New("please use a valid git url")
		return breverrors.WrapAndTrace(err)
	}

	temp := start.MakeNewWorkspaceFromURL(personalSettingsRepo)
	t.Vprint(temp.GitRepo)

	// TODO: make sure the git repo format works!!!!!!!

	_, err = profileStore.UpdateUser(user.ID, &entity.UpdateUser{
		Username:          user.Username,
		Name:              user.Name,
		Email:             user.Email,
		BaseWorkspaceRepo: temp.GitRepo,
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("Your personal config has been updated. All new dev environments will run this script.\n")
	return nil
}
