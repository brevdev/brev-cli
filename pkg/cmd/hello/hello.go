package hello

import (
	"fmt"
	"sync"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type HelloStore interface {
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetCurrentUser() (*entity.User, error)
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
}

func NewCmdHello(t *terminal.Terminal, store HelloStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "hello",
		DisableFlagsInUseLine: true,
		Long:                  "Get a quick onboarding of the Brev CLI",
		Short:                 "Get a quick onboarding of the Brev CLI",
		Example:               "brev hello",
		RunE: func(cmd *cobra.Command, args []string) error {
			// terminal.DisplayBrevLogo(t)
			t.Vprint("\n")

			user, err := store.GetCurrentUser()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			RunOnboarding(t, user, store)
			return nil
		},
	}

	return cmd
}

func TypeItToMe(s string) {
	sRunes := []rune(s)
	for i := 0; i < len(sRunes); i++ {
		// BANANA: undo this
		time.Sleep(7 * time.Millisecond)

		fmt.Printf("%c", sRunes[i])
	}
}

var wg sync.WaitGroup

func RunOnboarding(t *terminal.Terminal, user *entity.User, store HelloStore) {
	terminal.DisplayBrevLogo(t)
	t.Vprint("\n")

	s := "Hey " + GetFirstName(user.Name) + "!\n"

	TypeItToMe(s)

	s = "\n\nI'm Nader 👋  Co-founder of Brev. I'll show you around"
	s += "\nbtw, text me or call me if you need anything"
	s += ". My cell is " + t.Yellow("(415) 237-2247")
	TypeItToMe(s)

	s = "\n\nBrev is a dev tool for creating and sharing dev environments"
	TypeItToMe(s)

	wg.Add(2)
	go finishOutput(t)
	go MarkOnboardingStepCompleted(t, user, store)
	wg.Wait()
}

func finishOutput(t *terminal.Terminal) {
	s := "\nRun " + t.Green("brev ls") + " to see your dev environments 👇\n"
	TypeItToMe(s)
	wg.Done()
}

func MarkOnboardingStepCompleted(t *terminal.Terminal, user *entity.User, store HelloStore) {
	CompletedOnboardingIntro(user, store)
	wg.Done()
}
