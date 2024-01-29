package fu

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	stripmd "github.com/writeas/go-strip-markdown"
)

var (
	fuLong    string
	fuExample = "brev fu <user_id>"
)

type FuStore interface {
	completions.CompletionStore
	DeleteWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	BanUser(userID string) error
	GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error)
	GetAllOrgsAsAdmin(userID string) ([]entity.Organization, error)
}

func NewCmdFu(t *terminal.Terminal, loginFuStore FuStore, noLoginFuStore FuStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "fu",
		DisableFlagsInUseLine: true,
		Short:                 "Fetch all workspaces for a user and delete them",
		Long:                  stripmd.Strip(fuLong),
		Example:               fuExample,
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginFuStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			var allError error
			for _, userID := range args {
				err := fuUser(userID, t, loginFuStore)
				if err != nil {
					allError = multierror.Append(allError, err)
				}
			}
			if allError != nil {
				return breverrors.WrapAndTrace(allError)
			}
			return nil
		},
	}

	return cmd
}

func fuUser(userID string, t *terminal.Terminal, fuStore FuStore) error {
	orgs, err := fuStore.GetAllOrgsAsAdmin(userID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var allWorkspaces []entity.Workspace
	for _, org := range orgs {
		workspaces, err := fuStore.GetWorkspaces(org.ID, nil)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		allWorkspaces = append(allWorkspaces, workspaces...)
	}

	s := t.NewSpinner()
	s.Suffix = " Fetching workspaces for user " + userID
	s.Start()
	time.Sleep(5 * time.Second)
	s.Stop()

	confirm := terminal.PromptGetInput(terminal.PromptContent{
		Label:      fmt.Sprintf("Are you sure you want to delete all %d workspaces for user %s? (y/n)", len(allWorkspaces), userID),
		ErrorMsg:   "You must confirm to proceed.",
		AllowEmpty: false,
	})
	if confirm != "y" {
		return nil
	}

	for _, workspace := range allWorkspaces {
		_, err2 := fuStore.DeleteWorkspace(workspace.ID)
		if err2 != nil {
			t.Vprintf(t.Red("Failed to delete workspace with ID: %s\n", workspace.ID))
			t.Vprintf(t.Red("Error: %s\n", err.Error()))
			continue
		}
		t.Vprintf("✅ Deleted workspace %s\n", workspace.Name)
	}

	err = fuStore.BanUser(userID)
	if err != nil {
		t.Vprintf(t.Red("Failed to ban user with ID: %s\n", userID))
		t.Vprintf(t.Red("Error: %s\n", err.Error()))
	}
	t.Vprint("\n")
	t.Vprintf("🖕 Banned user %s\n", userID)

	return nil
}
