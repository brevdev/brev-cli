// Package ollama is for starting AI/ML model workspaces
package ollama

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/collections"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var (
	ollamaLong    = "Start an Ollama server with specified model types"
	ollamaExample = `
  brev ollama --model llama3
	`
	modelTypes = []string{"llama3"}
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
	ModifyPublicity(workspace *entity.Workspace, applicationName string, publicity bool) (*entity.Tunnel, error)
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

			// Start the network call in a goroutine
			res := collections.Async(func() (any, error) {
				err := runOllamaWorkspace(t, model, ollamaStore)
				if err != nil {
					return nil, breverrors.WrapAndTrace(err)
				}
				return nil, nil
			})

			// Type out the creating workspace message
			hello.TypeItToMeUnskippable27("🤙🦙🤙🦙🤙🦙🤙")
			t.Vprint(t.Green("\n\n\n"))

			_, err := res.Await()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", "", "AI/ML model type (e.g., llama2, llama3, mistral7b)")
	return cmd
}

func runOllamaWorkspace(t *terminal.Terminal, model string, ollamaStore OllamaStore) error { //nolint:funlen // todo
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

	s := t.NewSpinner()

	createWorkspaceRes := collections.Async(func() (*entity.Workspace, error) {
		w, errr := ollamaStore.CreateWorkspace(org.ID, cwOptions)
		if errr != nil {
			return nil, breverrors.WrapAndTrace(errr)
		}
		return w, nil
	})

	s.Suffix = " Creating your workspace. Hang tight 🤙"
	s.Start()

	w, err := createWorkspaceRes.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
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

	verbBuildRes := collections.Async(func() (*store.BuildVerbRes, error) {
		lf, errr := ollamaStore.BuildVerbContainer(w.ID, verbYaml)
		if errr != nil {
			return nil, breverrors.WrapAndTrace(errr)
		}
		return lf, nil
	})

	s.Start()
	s.Suffix = "Starting the Ollama server. Hang tight 🤙"

	_, err = verbBuildRes.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
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

	// Reload workspace to get the latest status
	w, err = ollamaStore.GetWorkspace(w.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	link, name, err := getOllamaTunnelLink(w)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = makeTunnelPublic(w, name, ollamaStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Print("\n")
	t.Vprint(t.Green("Ollama is ready to go!\n"))
	displayOllamaConnectBreadCrumb(t, link)
	return nil
}

func displayOllamaConnectBreadCrumb(t *terminal.Terminal, link string) {
	t.Vprintf(t.Green("Query the Ollama API with the following command:\n"))
	t.Vprintf(t.Yellow(fmt.Sprintf("curl %s/api/chat -d '{\n  \"model\": \"llama3\",\n  \"messages\": [\n    {\n      \"role\": \"user\",\n      \"content\": \"why is the sky blue?\"\n    }\n  ]\n}'", link)))
}

func pollInstanceUntilVMReady(workspace *entity.Workspace, interval time.Duration, timeout time.Duration, ollamaStore OllamaStore) (bool, error) {
	elapsedTime := time.Duration(0)

	for elapsedTime < timeout {
		w, err := ollamaStore.GetWorkspace(workspace.ID)
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
	return false, breverrors.New("Timeout waiting for machine to start")
}

func pollInstanceUntilVerbContainerReady(workspace *entity.Workspace, interval time.Duration, timeout time.Duration, ollamaStore OllamaStore) (bool, error) {
	elapsedTime := time.Duration(0)

	for elapsedTime < timeout {
		w, err := ollamaStore.GetWorkspace(workspace.ID)
		if err != nil {
			return false, breverrors.WrapAndTrace(err)
		} else if w.VerbBuildStatus == entity.Completed {
			return true, nil
		}
		time.Sleep(interval)
		elapsedTime += interval
	}
	return false, breverrors.New("Timeout waiting for container to build")
}

func getOllamaTunnelLink(workspace *entity.Workspace) (string, string, error) {
	for _, v := range workspace.Tunnel.Applications {
		if v.Port == 11434 {
			return v.Hostname, v.Name, nil
		}
	}
	return "", "", breverrors.New("Could not find Ollama tunnel")
}

// TODO: stub for granular permissioning
// func generateCloudflareAPIKeys(workspace *entity.Workspace, ollamaStore OllamaStore) (bool, error) {
// 	return false, nil
// }

func makeTunnelPublic(workspace *entity.Workspace, applicationName string, ollamaStore OllamaStore) (bool, error) {
	t, err := ollamaStore.ModifyPublicity(workspace, applicationName, true)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	for _, v := range t.Applications {
		if v.Port == 11434 {
			if v.Policy.AllowEveryone {
				return true, nil
			}
		}
	}
	return false, breverrors.New("Could not find Ollama tunnel")
}
