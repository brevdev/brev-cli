package profile

import (
	"errors"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/start"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"

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
		Args:                  cobra.NoArgs,
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginProfileStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			err := profile(personalSettingsRepo, t, loginProfileStore)
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}
		},
	}

	cmd.Flags().StringVarP(&personalSettingsRepo, "set-personal-config", "p", "", "set your personal config repo")

	return cmd
}

func profile(personalSettingsRepo string, t *terminal.Terminal, profileStore ProfileStore) error {
	user, err := profileStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
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

	t.Vprintf("Your personal config has been updated. All new workspaces will run this script.\n")
	return nil
}
