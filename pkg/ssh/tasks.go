package ssh

import (
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
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
	configs := []Config{
		NewSSHConfigurerV2(
			sct.Store,
		),
		NewSSHConfigurerJetBrains(sct.Store),
	}
	if featureflag.ServiceMeshSSH() {
		configs = []Config{
			NewSSHConfigurerServiceMesh(sct.Store),
			NewSSHConfigurerJetBrains(sct.Store),
		}
	}
	cu := ConfigUpdater{
		Store:   sct.Store,
		Configs: configs,
	}
	err := tasks.RunTasks([]tasks.Task{cu})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (sct SSHConfigurerTask) Configure() error {
	daemonConfigurer := autostartconf.NewSSHConfigurer(sct.Store)
	err := daemonConfigurer.Install()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func NewSSHConfigurerTask(store SSHConfigurerTaskStore) SSHConfigurerTask {
	return SSHConfigurerTask{
		Store: store,
	}
}
