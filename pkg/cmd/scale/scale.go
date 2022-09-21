package scale

import (
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var (
	long    = "Scale your Brev environment to get a more powerful machine or save costs"
	example = `
  brev scale MyDevEnvironment --gpu p3.2xlarge
  brev scale MyDevEnvironment --cpu 2x8
	`
	// TODO: we might wanna use the below for validation ¯\_(ツ)_/¯
	// instanceTypes = []string{"p4d.24xlarge", "p3.2xlarge", "p3.8xlarge", "p3.16xlarge", "p3dn.24xlarge", "p2.xlarge", "p2.8xlarge", "p2.16xlarge", "g5.xlarge", "g5.2xlarge", "g5.4xlarge", "g5.8xlarge", "g5.16xlarge", "g5.12xlarge", "g5.24xlarge", "g5.48xlarge", "g5g.xlarge", "g5g.2xlarge", "g5g.4xlarge", "g5g.8xlarge", "g5g.16xlarge", "g5g.metal", "g4dn.xlarge", "g4dn.2xlarge", "g4dn.4xlarge", "g4dn.8xlarge", "g4dn.16xlarge", "g4dn.12xlarge", "g4dn.metal", "g4ad.xlarge", "g4ad.2xlarge", "g4ad.4xlarge", "g4ad.8xlarge", "g4ad.16xlarge", "g3s.xlarge", "g3.4xlarge", "g3.8xlarge", "g3.16xlarge"}
)

type ScaleStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	ModifyWorkspace(organizationID string, options *store.ModifyWorkspaceRequest) (*entity.Workspace, error)
}

func NewCmdScale(t *terminal.Terminal, sstore ScaleStore) *cobra.Command {
	// var instanceType string
	var gpu string
	var cpu string

	cmd := &cobra.Command{
		Use:                   "scale",
		DisableFlagsInUseLine: true,
		Short:                 "Scale your Brev environment",
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return breverrors.NewValidationError("You must provide an instance with flage -i")
			}

			err := Runscale(t, args, gpu, cpu, sstore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&gpu, "gpu", "g", "", "GPU instance type.  See https://docs.brev.dev/docs/reference/gpu/#gpu-instance-types for details")
	cmd.Flags().StringVarP(&cpu, "cpu", "c", "", "CPU instance type.  See https://docs.brev.dev/docs/reference/gpu/#cpu-instance-types for details")
	// cmd.Flags().StringVarP(&instanceType, "instance", "i", "", "GPU or CPU instance type.  See docs.brev.dev/gpu for details")
	return cmd
}

func Runscale(t *terminal.Terminal, args []string, gpu string, cpu string, sstore ScaleStore) error {
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(sstore, args[0])
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var modifyBody store.ModifyWorkspaceRequest

	if gpu != "" {
		modifyBody.InstanceType = gpu
		t.Vprintf("\nScaling %s to %s GPU\n", workspace.Name, gpu)
	} else if cpu != "" {
		modifyBody.WorkspaceClassID = cpu
		t.Vprintf("\nScaling %s to %s CPU\n", workspace.Name, cpu)
	}

	ws, err := sstore.ModifyWorkspace(workspace.ID, &modifyBody)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprintf("\n%s scaled to %s\n", ws.Name, ws.InstanceType)

	return nil
}
