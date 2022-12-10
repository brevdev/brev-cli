package autostop

import (
	"github.com/hashicorp/go-multierror"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

func NewCmdautostop(t *terminal.Terminal, store autostopStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "autostop",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			return breverrors.WrapAndTrace(
				Runautostop(
					runAutostopArgs{
						t:     t,
						args:  args,
						store: store,
					},
				))
		},
	}
	return cmd
}

type autostopStore interface {
	AutoStopWorkspace(workspaceID string) (*entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error)
}

type runAutostopArgs struct {
	t     *terminal.Terminal
	args  []string
	store autostopStore
}

func Runautostop(args runAutostopArgs) error {
	org, err := args.store.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	envs, err := args.store.GetWorkspaceByNameOrID(org.ID, args.args[0])
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	asResults := lo.Map(
		envs,
		func(env entity.Workspace, _ int) mo.Result[*entity.Workspace] {
			return mo.TupleToResult(args.store.AutoStopWorkspace(env.ID))
		},
	)
	return breverrors.WrapAndTrace(
		lo.Reduce(
			asResults,
			func(acc error, res mo.Result[*entity.Workspace], _ int) error {
				return multierror.Append(acc, res.Error())
			},
			nil,
		),
	)
}
