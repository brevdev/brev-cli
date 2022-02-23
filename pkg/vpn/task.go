package vpn

import (
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/tasks"
)

type ServiceMeshStore interface {
	VPNStore
	GetCurrentWorkspaceID() string
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

type VPNDaemon struct {
	Store ServiceMeshStore
}

var _ tasks.Task = VPNDaemon{}

func (vpnd VPNDaemon) GetTaskSpec() tasks.TaskSpec {
	return tasks.TaskSpec{RunCronImmediately: true, Cron: ""}
}

func (vpnd VPNDaemon) Run() error {
	ts := NewTailscale(vpnd.Store)
	workspaceID := vpnd.Store.GetCurrentWorkspaceID()
	if workspaceID != "" {
		ts.WithUserspaceNetworking(true)
		ts.WithSockProxyPort(1055)
	}
	err := ts.Start()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
