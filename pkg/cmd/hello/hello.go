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

func NewCmdHello(_ *terminal.Terminal, store HelloStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "hello",
		DisableFlagsInUseLine: true,
		Long:                  "Get a quick onboarding of the Brev CLI",
		Short:                 "Get a quick onboarding of the Brev CLI",
		Example:               "brev hello",
		// Args:                  cmderrors.TransformToValidationError(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			// fmt.Println("ayo")
			TESTReadAndWriteFile()
			return nil
		},
	}

	return cmd
}

func TypeItToMe(s string) {
	sRunes := []rune(s)
	for i := 0; i < len(sRunes); i++ {
		// sleep for 100ms
		time.Sleep(47 * time.Millisecond)

		fmt.Printf("%c", sRunes[i])
	}
}

func RunOnboarding(t *terminal.Terminal) {
	s := "Welcome to Brev!"
	TypeItToMe(s)

	t.Vprint("\n")

	s = "Brev is a dev tool for creating and sharing dev environments"
	TypeItToMe(s)
}
