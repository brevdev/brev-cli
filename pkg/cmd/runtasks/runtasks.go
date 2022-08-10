package runtasks

import (
	_ "embed"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/ssh"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/vpn"
	"github.com/spf13/cobra"
	stripmd "github.com/writeas/go-strip-markdown"
)

//go:embed doc.md
var long string

func NewCmdRunTasks(t *terminal.Terminal, store RunTasksStore) *cobra.Command {
	var detached bool
	// would be nice to have a way to pass in a list of tasks to run instead of the default
	var runRemoteCMD bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "run-tasks",
		DisableFlagsInUseLine: true,
		Short:                 "Run background tasks for brev",
		Long:                  stripmd.Strip(long),
		Example:               "brev run-tasks -d",
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(0)),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunTasks(t, store, detached, runRemoteCMD)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&detached, "detached", "d", false, "run the command in the background instead of blocking the shell")
	cmd.Flags().BoolVarP(&runRemoteCMD, "run-remote-cmd", "r", true, "run the command on the environment to cd into ws default dir")
	return cmd
}

type RunTasksStore interface {
	ssh.ConfigUpdaterStore
	ssh.SSHConfigurerV2Store
	vpn.ServiceMeshStore
	tasks.RunTaskAsDaemonStore
	GetCurrentUser() (*entity.User, error)
	GetCurrentUserKeys() (*entity.UserKeys, error)
}

func RunTasks(_ *terminal.Terminal, store RunTasksStore, detached, runRemoteCMD bool) error {
	ts, err := getDefaultTasks(store, runRemoteCMD)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if detached {
		err := tasks.RunTaskAsDaemon(ts, store)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		err := tasks.RunTasks(ts)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func getDefaultTasks(store RunTasksStore, runRemoteCMD bool) ([]tasks.Task, error) {
	configs, err := ssh.GetSSHConfigs(store, runRemoteCMD)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	// get private key and set here
	workspaceGroupClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(store) // to resolve
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	privateKey := workspaceGroupClientMapper.GetPrivateKey()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	cu := ssh.ConfigUpdater{
		Store:      store,
		Configs:    configs,
		PrivateKey: privateKey,
	}

	return []tasks.Task{cu}, nil
}
