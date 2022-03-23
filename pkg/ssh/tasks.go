package ssh

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/tasks"
)

type SSHConfigurerTaskStore interface {
	ConfigUpdaterStore
	SSHConfigurerV2Store
}

type SSHConfigurerTask struct {
	Store SSHConfigurerTaskStore
}

var _ tasks.Task = SSHConfigurerTask{}

func (sct SSHConfigurerTask) GetTaskSpec() tasks.TaskSpec {
	return tasks.TaskSpec{RunCronImmediately: true, Cron: "@every 3s"}
}

func (sct SSHConfigurerTask) Run() error {
	cu := ConfigUpdater{
		Store: sct.Store,
		Configs: []Config{
			NewSSHConfigurerV2(
				sct.Store,
			),
			NewSSHConfigurerServiceMesh(sct.Store),
			NewSSHConfigurerJetBrains(sct.Store),
		},
	}
	err := tasks.RunTasks([]tasks.Task{cu})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (sct SSHConfigurerTask) Configure() error {
	return nil
}
