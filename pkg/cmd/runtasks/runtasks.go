package runtasks

import (
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/ssh"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/vpn"
	"github.com/spf13/cobra"
)

func NewCmdRunTasks(t *terminal.Terminal, store RunTasksStore) *cobra.Command {
	var detached bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "run-tasks",
		DisableFlagsInUseLine: true,
		Short:                 "Run background tasks for brev",
		Long:                  "Run tasks keeps the ssh config up to date and a background vpn daemon to connect you to your service mesh. Run with -d to run as a detached daemon in the background. To force a refresh to your config use the refresh command.",
		Example:               "brev run-tasks -d",
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(0)),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunTasks(t, store, detached)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&detached, "detached", "d", false, "run the command in the background instead of blocking the shell")

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

func RunTasks(_ *terminal.Terminal, store RunTasksStore, detached bool) error {
	ts, err := getDefaultTasks(store)
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

func getDefaultTasks(store RunTasksStore) ([]tasks.Task, error) {
	configs, err := ssh.GetSSHConfigs(store)
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
