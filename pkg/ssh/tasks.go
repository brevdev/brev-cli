package ssh

import "github.com/brevdev/brev-cli/pkg/tasks"

type SSHConfigurerTaskStore interface{}

type SSHConfigurerTask struct{}

var _ tasks.Task = SSHConfigurerTask{}

func (sct SSHConfigurerTask) GetTaskSpec() tasks.TaskSpec {
	return tasks.TaskSpec{RunCronImmediately: true, Cron: "@every 3s"}
}

func (sct SSHConfigurerTask) Run() error {
	return nil
}

func (sct SSHConfigurerTask) Configure() error {
	return nil
}
