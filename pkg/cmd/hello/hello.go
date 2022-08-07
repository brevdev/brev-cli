package hello

import (
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type HelloStore interface {
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
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
			RunOnboarding(t)
			return nil
		},
	}

	return cmd
}

func TypeItToMe(s string) {
	sRunes := []rune(s)
	for i := 0; i < len(sRunes); i++ {
		// sleep for 100ms
		// BANANA: put this back to 47
		time.Sleep(17 * time.Millisecond)

		fmt.Printf("%c", sRunes[i])
	}
}

func RunOnboarding(t *terminal.Terminal) {
	terminal.DisplayBrevLogo(t)
	t.Vprint("\n")

	s := "Welcome to Brev!"
	TypeItToMe(s)

	s = "\nBrev is a dev tool for creating and sharing dev environments"
	TypeItToMe(s)

	s = "\n\nI'm Nader 👋  Co-founder of Brev. I'll show you around"
	s += "\nbtw, text me or call me if you need anything"
	s += ". My cell is " + t.Yellow("(415) 237-2247")
	TypeItToMe(s)

	s = "\n\nRun " + t.Green("brev ls") + " to see your dev environments 👇\n"
	TypeItToMe(s)
}
