// Package ollama is for starting AI/ML model workspaces
package ollama

import (
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var (
	ollamaLong    = "Start an AI/ML model workspace with specified model types"
	ollamaExample = `
  brev ollama --model llama2
  brev ollama --model mistral7b
	`
	modelTypes = []string{"llama2", "llama3", "mistral7b"}
)

type OllamaStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
}

func validateModelType(modelType string) bool {
	for _, v := range modelTypes {
		if modelType == v {
			return true
		}
	}
	return false
}

func NewCmdOllama(t *terminal.Terminal, ollamaStore OllamaStore) *cobra.Command {
	var model string

	cmd := &cobra.Command{
		Use:                   "ollama",
		DisableFlagsInUseLine: true,
		Short:                 "Start an AI/ML model workspace",
		Long:                  ollamaLong,
		Example:               ollamaExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			if model == "" {
				return fmt.Errorf("model type must be specified")
			}

			isValid := validateModelType(model)
			if !isValid {
				return fmt.Errorf("invalid model type: %s", model)
			}

			// Channel to get the result of the network call
			resultCh := make(chan error)

			// Start the network call in a goroutine
			go func() {
				err := runOllamaWorkspace(t, model, ollamaStore)
				resultCh <- err
			}()

			// Type out the creating workspace message
			hello.TypeItToMeUnskippable27("🤙🦙🤙🦙🤙🦙🤙")
			t.Vprint(t.Green("\n\n\n"))

			// Wait for the network call to finish
			err := <-resultCh
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", "", "AI/ML model type (e.g., llama2, llama3, mistral7b)")
	return cmd
}

func runOllamaWorkspace(t *terminal.Terminal, model string, ollamaStore OllamaStore) error {
	_, err := ollamaStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	org, err := ollamaStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Placeholder for instance type, to be updated later
	instanceType := "A100-Placeholder"

	cwOptions := store.NewCreateWorkspacesOptions("default-cluster", model).WithInstanceType(instanceType)

	// Type out the creating workspace message
	hello.TypeItToMeUnskippable27(fmt.Sprintf("Creating AI/ML workspace %s with model %s in org %s", t.Green(cwOptions.Name), t.Green(model), t.Green(org.ID)))

	// Channel to get the result of the network call
	resultCh := make(chan error)

	dev := false

	// Start the network call in a goroutine
	go func() {
		var err error
		if !dev {
			_, err = ollamaStore.CreateWorkspace(org.ID, cwOptions)
		}
		resultCh <- err
	}()

	s := t.NewSpinner()
	s.Suffix = " Creating your workspace. Hang tight 🤙"

	// Wait for the network call to finish or timeout for spinner start
	select {
	case err := <-resultCh:
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	case <-time.After(2 * time.Second):
		s.Start()
	}

	// Wait for the network call to finish if not already done
	if !dev {
		err := <-resultCh
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		time.Sleep(3 * time.Second) // Simulate delay in development mode
	}

	s.Stop()

	fmt.Print("\n")
	t.Vprint(t.Green("Your AI/ML workspace is ready!\n"))
	// displayConnectBreadCrumb(t, w)

	return nil
}

// func displayConnectBreadCrumb(t *terminal.Terminal, workspace *entity.Workspace) {
// 	t.Vprintf(t.Green("Connect to the workspace:\n"))
// 	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev open %s\t# brev open <NAME> -> open workspace in VS Code\n", workspace.Name)))
// 	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev shell %s\t# brev shell <NAME> -> ssh into workspace (shortcut)\n", workspace.Name)))
// }
