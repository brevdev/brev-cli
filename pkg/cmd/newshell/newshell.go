// Package newshell to prompt the user for which shell to use
package newshell

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type NewShellStore interface {
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetUsers(queryParams map[string]string) ([]entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
}

func NewCmdNewShell(t *terminal.Terminal, loginLsStore NewShellStore, noLoginLsStore NewShellStore) *cobra.Command {

	cmd := &cobra.Command{
		Annotations: map[string]string{"context": ""},
		Use:         "new-shell",
		Short:       "Prompt for which shell to run",
		Long:        "Get a prompt for which shell to run",
		Example: `
  brev new-shell
		`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		Args:      cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunNewShell(t, loginLsStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	return cmd
}

func RunNewShell(t *terminal.Terminal, lsStore NewShellStore) error {
	user, err := lsStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var org *entity.Organization
	org, err = lsStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return fmt.Errorf("no orgs exist")
	}

	var workspaces []entity.Workspace
	workspaces, err = lsStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{UserID: user.ID})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(workspaces) >= 0 {

		wsmap := getListOfLocalIdentifiers(workspaces)
		var workspaceIdentifiers []string
		for k, _ := range wsmap { 
			workspaceIdentifiers = append(workspaceIdentifiers, k)

		}
		workspaceIdentifiers = append(workspaceIdentifiers, "your local shell")

		// DO THE PROMPT HERE!!!!!
		selectedWorkspace := terminal.PromptSelectInput(terminal.PromptSelectContent{
			Label:    "Type of variable: ",
			ErrorMsg: "error",
			Items:    workspaceIdentifiers,
		})

		if selectedWorkspace == "your local shell" {
			t.Vprint("done!")
		} else {
			t.Vprint(t.Green("You selected %s w/ ID %s and URL %s", selectedWorkspace, wsmap[selectedWorkspace].ID, wsmap[selectedWorkspace].DNS))
		}
	}

	return nil
}

func getListOfLocalIdentifiers(workspaces []entity.Workspace) map[string]entity.Workspace {
	var res map[string]entity.Workspace

	if len(workspaces) >0 {
		var localIdentifiers []string
		for _, v := range workspaces {
			li := string(v.GetLocalIdentifier(workspaces))
			localIdentifiers = append(localIdentifiers, li)
			res[li] = v

		}
		return res

	} else {
		return nil
	}

	return nil
}