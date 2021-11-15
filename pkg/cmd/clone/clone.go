package clone

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

func isValidGitUrl(url string) bool {
	return true
}

func NewCmdClone(t *terminal.Terminal) *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Annotations: map[string]string{"workspace": ""},
		Use:         "clone",
		Short:       "Clone a git repo into a fresh workspace",
		Long:        "Create a workspace by repo url",
		Example:     `  brev clone <url>`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
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
			err := clone(t, args[0], org)
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org)")
	cmd.RegisterFlagCompletionFunc("org", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return brevapi.GetOrgNames(), cobra.ShellCompDirectiveNoSpace
	})
	return cmd
}

// "https://github.com/brevdev/microservices-demo.git
// "https://github.com/brevdev/microservices-demo.git"
// "git@github.com:brevdev/microservices-demo.git"
func clone(t *terminal.Terminal, url string, orgflag string) error {
	formattedURL := validateGitUrl(t, url)

	var orgID string
	if orgflag == "" {
		activeorg, err := brevapi.GetActiveOrgContext(files.AppFs)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		orgID = activeorg.ID
	} else {
		org, err := brevapi.GetOrgFromName(orgflag)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		orgID = org.ID
	}

	err := createWorkspace(t, formattedURL, orgID)
	if err != nil {
		t.Vprint(t.Red(err.Error()))
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

func createWorkspace(t *terminal.Terminal, newworkspace NewWorkspace, orgID string) error {
	c, err := brevapi.NewClient()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	w, err := c.CreateWorkspace(orgID, newworkspace.Name, newworkspace.GitRepo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprint(t.Green("Cloned workspace at %s", w.DNS))

	return nil
}
