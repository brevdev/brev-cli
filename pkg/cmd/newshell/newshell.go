// Package newshell to prompt the user for which shell to use
package newshell

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	ssh "github.com/nanobox-io/golang-ssh"

	"github.com/spf13/cobra"
)

type NewShellStore interface {
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetUsers(queryParams map[string]string) ([]entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetPrivateKeyPath() (string, error)
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
	if len(workspaces) > 0 {

		var workspaceNames []string
		for _, v := range workspaces { 
			if v.Status == "RUNNING" {
				workspaceNames = append(workspaceNames, v.Name)
			}
		}
		workspaceNames = append(workspaceNames, "your local shell")

		selectedWorkspaceName := terminal.PromptSelectInput(terminal.PromptSelectContent{
			Label:    "Which shell do you want to open? ",
			ErrorMsg: "error",
			Items:    workspaceNames,
		})
		
		var workspace *entity.Workspace
		if selectedWorkspaceName == "your local shell" {
			t.Vprint("done!")
		} else {
			for _, v := range workspaces {
				if v.Name == selectedWorkspaceName {
					workspace = &v
					break
				}
			}
			if workspace == nil {
				return nil
			}

			t.Vprint(t.Green("You selected %s w/ ID %s and URL %s", selectedWorkspaceName, workspace.ID, workspace.DNS))
		}

		s,err := lsStore.GetPrivateKeyPath()
		if err != nil {
			t.Vprint(t.Red("Aborting"))
			return nil;
		}
		t.Vprint(t.Green("Path: %s", s))

		ws := *workspace
		wsLocalID := string(ws.GetLocalIdentifier(workspaces))
		err = connect(wsLocalID, []string{s})
		if err != nil {
			t.Vprintf(err.Error())
			return nil
		}
		
	}

	return nil
}

func connect(localIdentifier string, paths []string) error {
	nanPass := ssh.Auth{Keys: paths}
	client, err := ssh.NewNativeClient("brev", localIdentifier, "", 22, &nanPass, nil)
	if err != nil {
		return fmt.Errorf("Failed to create new client - %s", err)
	}

	err = client.Shell()
	if err != nil && err.Error() != "exit status 255" {
		return fmt.Errorf("Failed to request shell - %s", err)
	}

	return nil
}