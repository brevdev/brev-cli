package vpn

import (
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/tasks"
)

type ServiceMeshStore interface {
	VPNStore
	GetNetworkAuthKey() (*store.GetAuthKeyResponse, error)
	GetCurrentWorkspaceID() (string, error)
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
	workspaceID, err := vpnd.Store.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		ts.WithUserspaceNetworking(true).
			WithSockProxyPort(1055)
	}
	err = ts.Start()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func ConfigureVPN(store ServiceMeshStore) error {
	ts := NewTailscale(store)
	nodeIdentifier := "me"
	workspaceID, err := store.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		var workspace *entity.Workspace
		workspace, err = store.GetWorkspace(workspaceID)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		nodeIdentifier = workspace.GetNodeIdentifierForVPN(nil)
	}
	authKeyResp, err := store.GetNetworkAuthKey()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	tsc := ts.WithConfigurerOptions(nodeIdentifier, authKeyResp.CoordServerURL).WithForceReauth(true)
	tsca := tsc.WithAuthKey(authKeyResp.AuthKey)
	err = tsca.ApplyConfig()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
