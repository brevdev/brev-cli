// Package ollama is for starting AI/ML model workspaces
package ollama

import (
	_ "embed"
	"fmt"
	"time"

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

//go:embed ollamaverb.yaml
var verbYaml string

type OllamaStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
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

	hello.TypeItToMeUnskippable27(fmt.Sprintf("Creating Ollama server %s with model %s in org %s", t.Green(cwOptions.Name), t.Green(model), t.Green(org.ID)))

	w := &entity.Workspace{}
	workspaceCh := make(chan *entity.Workspace)

	go func() {
		w, err = ollamaStore.CreateWorkspace(org.ID, cwOptions)
		if err != nil {
			// ideally log something here?
			workspaceCh <- nil
			return
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

	var vmStatus bool
	vmStatus, err = pollInstanceUntilVMReady(w, time.Second, time.Minute*5, ollamaStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if !vmStatus {
		return breverrors.New("instance did not start")
	}

	// sleep for 10 seconds to solve for possible race condition
	// TODO: look into timing of verb call
	time.Sleep(time.Second * 10)

	lf := &store.BuildVerbRes{}
	verbCh := make(chan *store.BuildVerbRes)
	go func() {
		lf, err = ollamaStore.BuildVerbContainer(w.ID, verbYaml)
		if err != nil {
			// ideally log something here?
			verbCh <- nil
			return
		}
		verbCh <- lf
	}()

	s.Suffix = " Building your verb container. Hang tight 🤙"

	lf = <-verbCh
	if lf == nil {
		return breverrors.New("verb container build did not start")
	}

	var vstatus bool
	// TODO: 15 min for now because the image is not cached and takes a while to build. Remove this when the image is cached
	vstatus, err = pollInstanceUntilVerbContainerReady(w, time.Second, time.Minute*20, ollamaStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if !vstatus {
		return breverrors.New("verb container did not build correctly")
	}

	s.Stop()

	fmt.Print("\n")
	t.Vprint(t.Green("Your AI/ML workspace is ready!\n"))
	displayConnectBreadCrumb(t, w)
	return nil
}

func displayConnectBreadCrumb(t *terminal.Terminal, workspace *entity.Workspace) {
	t.Vprintf(t.Green("Connect to the Ollama server:\n"))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev open %s\t# brev open <NAME> -> open workspace in VS Code\n", workspace.Name)))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev shell %s\t# brev shell <NAME> -> ssh into workspace (shortcut)\n", workspace.Name)))
}

func pollInstanceUntilVMReady(workspace *entity.Workspace, interval time.Duration, timeout time.Duration, ollamaStore OllamaStore) (bool, error) {
	elapsedTime := time.Duration(0)

	for elapsedTime < timeout {
		w, err := ollamaStore.GetWorkspace(workspace.ID)
		fmt.Println(workspace.ID)
		fmt.Println(w.ID)
		fmt.Println(w.Status)
		if err != nil {
			return false, breverrors.WrapAndTrace(err)
		} else if w.Status == "RUNNING" {
			// adding a slight delay to make sure the instance is ready
			time.Sleep(time.Minute * 1)
			return true, nil
		}
		time.Sleep(interval)
		elapsedTime += interval
	}
	return false, breverrors.New("timeout waiting for instance to start")
}

func pollInstanceUntilVerbContainerReady(workspace *entity.Workspace, interval time.Duration, timeout time.Duration, ollamaStore OllamaStore) (bool, error) {
	elapsedTime := time.Duration(0)

	for elapsedTime < timeout {
		w, err := ollamaStore.GetWorkspace(workspace.ID)
		fmt.Println(workspace.ID)
		fmt.Println(w.ID)
		fmt.Println(w.VerbBuildStatus)
		if err != nil {
			return false, breverrors.WrapAndTrace(err)
		} else if w.VerbBuildStatus == entity.Completed {
			return true, nil
		}
		time.Sleep(interval)
		elapsedTime += interval
	}
	return false, breverrors.New("timeout waiting for instance to start")
}
