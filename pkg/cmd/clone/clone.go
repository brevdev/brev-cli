package clone

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

func isValidGitUrl(url string) bool {
	return true
}

func NewCmdClone(t *terminal.Terminal) *cobra.Command {

	cmd := &cobra.Command{
		Annotations: map[string]string{"ssh": ""},
		Use:         "clone",
		Short:       "clone a git repo",
		Long:        "Create a workspace by repo url",
		Example:     `  brev clone <url>`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return err
			}
			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires a git url")
			}

			if !isValidGitUrl(args[0]) {
				return errors.New("please use a valid git url")
			}
			return nil

		},
		RunE: func(cmd *cobra.Command, args []string) error {
			clone(t, args[0])
			return nil
		},
	}

	return cmd
}

// "https://github.com/brevdev/microservices-demo.git
// "https://github.com/brevdev/microservices-demo.git"
// "git@github.com:brevdev/microservices-demo.git"
func clone(t *terminal.Terminal, url string) error {
	formattedURL := validateGitUrl(t, url)

	err := createWorkspace(t, formattedURL)

	if err != nil {
		t.Vprint(err.Error())
	}
	return nil
}

type NewWorkspace struct {
	Name    string `json:"name"`
	GitRepo string `json:"gitRepo"`
}

func validateGitUrl(t *terminal.Terminal, url string) NewWorkspace {
	// gitlab.com:mygitlaborg/mycoolrepo.git
	if strings.Contains(url, "http") {
		split := strings.Split(url, ".com/")
		provider := strings.Split(split[0], "://")[1]

		if strings.Contains(split[1], ".git") {
			return NewWorkspace{
				GitRepo: fmt.Sprintf("%s.com:%s", provider, split[1]),
				Name:    strings.Split(split[1], ".git")[0],
			}
		} else {
			return NewWorkspace{
				GitRepo: fmt.Sprintf("%s.com:%s.git", provider, split[1]),
				Name:    split[1],
			}
		}
	} else {
		split := strings.Split(url, ".com:")
		provider := strings.Split(split[0], "@")[1]
		return NewWorkspace{
			GitRepo: fmt.Sprintf("%s.com:%s", provider, split[1]),
			Name:    strings.Split(split[1], ".git")[0],
		}
	}
}

func createWorkspace(t *terminal.Terminal, newworkspace NewWorkspace) error {
	activeorg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		return err
	}

	c, err := brev_api.NewClient()
	if err != nil {
		return err
	}

	w, err := c.CreateWorkspace(activeorg.ID, newworkspace.Name, newworkspace.GitRepo)
	if err != nil {
		return err
	}
	t.Vprint(t.Green("Cloned workspace at %s", w.DNS))

	return nil
}
