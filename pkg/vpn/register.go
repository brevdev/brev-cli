package vpn

import (
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func RegisterNode(store ServiceMeshStore) error {
	ts := NewTailscale(store)
	nodeIdentifier := "me"
	workspaceID := store.GetCurrentWorkspaceID()
	if workspaceID != "" {
		workspace, err := store.GetWorkspace(workspaceID)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		localIdentifier := workspace.GetLocalIdentifier(nil)
		nodeIdentifier = string(localIdentifier)
	}

	err := ts.ApplyConfig(nodeIdentifier, config.GlobalConfig.GetServiceMeshCoordServerURL())
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
