package reset

import (
	_ "embed"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/terminal"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/cobra"
	stripmd "github.com/writeas/go-strip-markdown"
)

var (
	//go:embed doc.md
	long         string
	startExample = `brev reset <ws_name>...`
)

type ResetStore interface {
	completions.CompletionStore
	util.GetWorkspaceByNameOrIDErrStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdReset(t *terminal.Terminal, loginResetStore ResetStore, noLoginResetStore ResetStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"provider-dependent": ""},
		Use:                   "reset",
		DisableFlagsInUseLine: true,
		Short:                 "Reset an instance to recover from errors",
		Long:                  stripmd.Strip(long),
		Example:               startExample,
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginResetStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				err := resetWorkspace(arg, t, loginResetStore)
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
			}
			return nil
		},
	}
	return cmd
}

func resetWorkspace(workspaceName string, t *terminal.Terminal, resetStore ResetStore) error {
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(resetStore, workspaceName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	startedWorkspace, err := resetStore.ResetWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("%s", t.Yellow("Instance %s is resetting.\n", startedWorkspace.Name))
	t.Vprintf("Note: this can take a few seconds. Run 'brev ls' to check status\n")

	return nil
}
