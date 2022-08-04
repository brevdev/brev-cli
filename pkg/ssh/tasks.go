package ssh

import (
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/tasks"
)

type SSHConfigurerTaskStore interface {
	ConfigUpdaterStore
	SSHConfigurerV2Store
	GetCurrentUser() (*entity.User, error)
}

type SSHConfigurerTask struct {
	Store SSHConfigurerTaskStore
}

var _ tasks.Task = SSHConfigurerTask{}

func (sct SSHConfigurerTask) GetTaskSpec() tasks.TaskSpec {
	return tasks.TaskSpec{RunCronImmediately: true, Cron: "@every 3s"}
}

func (sct SSHConfigurerTask) Run() error {
	configs, err := GetSSHConfigs(sct.Store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	cu := ConfigUpdater{
		Store:   sct.Store,
		Configs: configs,
	}
	err = tasks.RunTasks([]tasks.Task{cu})
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

func GetSSHConfigs(store SSHConfigurerTaskStore) ([]Config, error) {
	user, err := store.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	configs := []Config{
		NewSSHConfigurerV2(
			store,
		),
	}
	// if featureflag.ServiceMeshSSH(user.GlobalUserType) {
	// 	configs = []Config{
	// 		NewSSHConfigurerServiceMesh(store),
	// 	}
	// }
	jetbrainsConfigurer, err := NewSSHConfigurerJetBrains(store)
	// add jetbrainsconfigurer to configs if we can, but if we can't that
	// shouldn't prevent us from setting up the other configs
	if err == nil && jetbrainsConfigurer != nil {
		configs = append(configs, jetbrainsConfigurer)
	}
	return configs, nil
}
