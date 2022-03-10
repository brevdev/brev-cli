package vpn

import (
	"os/user"
	"runtime"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/spf13/afero"
)

type ServiceMeshStore interface {
	CopyBin(targetBin string) error
	WriteString(path, data string) error
	RegisterNode(publicKey string) error
	GetOrCreateFile(path string) (afero.File, error)
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

func (vpnd VPNDaemon) Configure(_ *user.User) error {
	switch runtime.GOOS {
	case "linux":
		err := vpnd.configureLinux()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	case "darwin":
		err := vpnd.configureDarwin()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func (vpnd VPNDaemon) configureLinux() error {
	lsc := autostartconf.NewVPNConfig(vpnd.Store)
	err := lsc.Install()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (vpnd VPNDaemon) configureDarwin() error { return nil }

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

func NewVPNDaemon(store ServiceMeshStore) VPNDaemon {
	vpnd := VPNDaemon{
		Store: store,
	}
	return vpnd
}
