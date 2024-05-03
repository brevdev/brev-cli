// Package ollama is for starting AI/ML model workspaces
package ollama

import (
	_ "embed"
	"fmt"

	"github.com/google/uuid"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
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

//go:embed verb.yaml
var verbYaml string

type OllamaStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	BuildVerbContainer(workspaceID string, verbYaml string) (*store.BuildVerbRes, error)
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
		Short:                 "Start an Ollama model server",
		Long:                  ollamaLong,
		Example:               ollamaExample,
		Annotations: map[string]string{
			"quickstart": "",
		},
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
			defer close(resultCh) // TODO: im thinking this is needed?

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
	instanceType := "g4dn.xlarge"
	clusterID := config.GlobalConfig.GetDefaultClusterID()
	uuid := uuid.New().String()
	instanceName := fmt.Sprintf("ollama-%s", uuid)
	cwOptions := store.NewCreateWorkspacesOptions(clusterID, instanceName).WithInstanceType(instanceType)

	// Type out the creating workspace message
	hello.TypeItToMeUnskippable27(fmt.Sprintf("Creating AI/ML workspace %s with model %s in org %s", t.Green(cwOptions.Name), t.Green(model), t.Green(org.ID)))

	// Channel to get the result of the network call
	w := &entity.Workspace{}
	workspaceCh := make(chan *entity.Workspace)

	dev := false

	// Start the network call in a goroutine
	go func() {
		var err error
		if !dev {
			w, err = ollamaStore.CreateWorkspace(org.ID, cwOptions)
			if err != nil {
				// ideally log something here?
				workspaceCh <- nil
				return
			}
		}
		workspaceCh <- w
	}()

	s := t.NewSpinner()
	s.Suffix = " Creating your workspace. Hang tight 🤙"
	s.Start()

	w = <-workspaceCh
	if w == nil {
		return breverrors.New("workspace creation failed")
	}

	// TODO: i need to check logic here. Should build verb ONLY happen after the machine is running?
	lf := &store.BuildVerbRes{}
	verbCh := make(chan *store.BuildVerbRes)
	// Start the network call to build the verb container
	go func() {
		var err error
		if !dev {
			lf, err = ollamaStore.BuildVerbContainer(w.ID, verbYaml)
			if err != nil {
				// ideally log something here?
				verbCh <- nil
				return
			}
		}
		verbCh <- lf
	}()

	select {
	case lf = <-verbCh:
		if lf == nil {
			return breverrors.New("verb container build failed")
		}
	}

	s.Suffix = " Building your verb container. Hang tight 🤙"
	s.Stop()

	fmt.Print("\n")
	t.Vprint(t.Green("Your AI/ML workspace is ready!\n"))
	// displayConnectBreadCrumb(t, workspace)
	return nil
}

// func displayConnectBreadCrumb(t *terminal.Terminal, workspace *entity.Workspace) {
// 	t.Vprintf(t.Green("Connect to the workspace:\n"))
// 	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev open %s\t# brev open <NAME> -> open workspace in VS Code\n", workspace.Name)))
// 	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev shell %s\t# brev shell <NAME> -> ssh into workspace (shortcut)\n", workspace.Name)))
// }
