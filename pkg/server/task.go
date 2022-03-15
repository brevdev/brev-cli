package server

import (
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/brevdev/brev-cli/pkg/vpn"
)

type RPCServerTaskStore interface {
	vpn.ServiceMeshStore
	GetServerSockFile() string
}

type RPCServerTask struct {
	Store RPCServerTaskStore
}

var _ tasks.Task = RPCServerTask{}

func (rst RPCServerTask) GetTaskSpec() tasks.TaskSpec {
	return tasks.TaskSpec{RunCronImmediately: true, Cron: ""}
}

func (rst RPCServerTask) Run() error {
	sock := rst.Store.GetServerSockFile()
	server := NewServer(sock, rst.Store)
	err := server.Serve()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (rst RPCServerTask) Configure() error {
	lsc := autostartconf.NewRPCConfig(rst.Store)
	err := lsc.Install()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func NewRPCServerTask(store RPCServerTaskStore) RPCServerTask {
	task := RPCServerTask{
		Store: store,
	}
	return task
}
