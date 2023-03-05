package status

import (
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	createLong    = "Create a new Brev machine"
	createExample = `
  brev create <name>
	`
	// instanceTypes = []string{"p4d.24xlarge", "p3.2xlarge", "p3.8xlarge", "p3.16xlarge", "p3dn.24xlarge", "p2.xlarge", "p2.8xlarge", "p2.16xlarge", "g5.xlarge", "g5.2xlarge", "g5.4xlarge", "g5.8xlarge", "g5.16xlarge", "g5.12xlarge", "g5.24xlarge", "g5.48xlarge", "g5g.xlarge", "g5g.2xlarge", "g5g.4xlarge", "g5g.8xlarge", "g5g.16xlarge", "g5g.metal", "g4dn.xlarge", "g4dn.2xlarge", "g4dn.4xlarge", "g4dn.8xlarge", "g4dn.16xlarge", "g4dn.12xlarge", "g4dn.metal", "g4ad.xlarge", "g4ad.2xlarge", "g4ad.4xlarge", "g4ad.8xlarge", "g4ad.16xlarge", "g3s.xlarge", "g3.4xlarge", "g3.8xlarge", "g3.16xlarge"}
)

type StatusStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentWorkspaceID() (string, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
}

func NewCmdStatus(t *terminal.Terminal, statusStore StatusStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "status",
		DisableFlagsInUseLine: true,
		Short:                 "About this instance",
		Long:                  createLong,
		Example:               createExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			runShowStatus(t, statusStore)
			return nil
		},
	}
	return cmd
}

func runShowStatus(t *terminal.Terminal, statusStore StatusStore) {
	terminal.DisplayBrevLogo(t)
	t.Vprintf("\n")
	wsID, err := statusStore.GetCurrentWorkspaceID()
	if err != nil {
		t.Vprintf("\n Error: %s", t.Red(err.Error()))
		return
	}
	ws, err := statusStore.GetWorkspace(wsID)
	if err != nil {
		t.Vprintf("\n Error: %s", t.Red(err.Error()))
		return
	}

	t.Vprintf("\nYou're on environment %s", t.Yellow(ws.Name))
	t.Vprintf("\n\tID: %s", t.Yellow(ws.ID))
	t.Vprintf("\n\tMachine: %s", t.Yellow(util.GetInstanceString(*ws)))
}
