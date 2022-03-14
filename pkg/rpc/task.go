package rpcserver

import (
	"os/user"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/brevdev/brev-cli/pkg/vpn"
)

type RPCServerTaskStore interface {
	vpn.ServiceMeshStore
}

type RPCServerTask struct {
	Store RPCServerTaskStore
}

var _ tasks.Task = RPCServerTask{}

func (rst RPCServerTask) GetTaskSpec() tasks.TaskSpec {
	return tasks.TaskSpec{RunCronImmediately: true, Cron: ""}
}

func (rst RPCServerTask) Run() error {
	server := NewServer(rst.Store, "/tmp/brev.sock") // todo /var/run
	err := server.Serve()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (rst RPCServerTask) Configure(_ *user.User) error {
	return nil
}

func NewRPCServerTask(store RPCServerStore) RPCServerTask {
	task := RPCServerTask{
		Store: store,
	}
	return task
}
