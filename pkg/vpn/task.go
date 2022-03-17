package vpn

import (
	"fmt"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/tasks"
)

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
	VPNStore
	GetNetworkAuthKey() (*store.GetAuthKeyResponse, error)
	GetCurrentWorkspaceID() (string, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetNetwork(networkID string) (*store.GetNetworkResponse, error)
	GetActiveOrganizationOrNil() (*entity.Organization, error)
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

func (vpnd VPNDaemon) Configure() error {
	daemonConfigurer := autostartconf.NewVPNConfig(vpnd.Store)
	err := daemonConfigurer.Install()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = ConfigureVPN(vpnd.Store)
	if err != nil {
		// if can't connect to tailscale try again, and if error is not auth return it
		if strings.Contains(err.Error(), "failed to connect to local Tailscale service") {
			fmt.Println("failed to connect to vpn trying again")
			time.Sleep(2 * time.Second)
			err = ConfigureVPN(vpnd.Store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
		} else if !auth.IsAuthError(err) {
			return breverrors.WrapAndTrace(err)
		}
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

	networkdID := ""
	if workspaceID != "" {
		var workspace *entity.Workspace
		workspace, err = store.GetWorkspace(workspaceID)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		nodeIdentifier = workspace.GetNodeIdentifierForVPN(nil)
		networkdID = workspace.NetworkID
	}
	authKeyResp, err := store.GetNetworkAuthKey()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if networkdID == "" {
		var org *entity.Organization
		org, err = store.GetActiveOrganizationOrNil()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if org == nil {
			return fmt.Errorf("no org exists")
		}
		networkdID = org.UserNetworkID
	}

	network, err := store.GetNetwork(networkdID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	tsc := ts.WithConfigurerOptions(nodeIdentifier, authKeyResp.CoordServerURL).WithForceReauth(true).WithSearchDomains(network.DNSSearchDomains)
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
