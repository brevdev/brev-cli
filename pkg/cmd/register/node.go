package register

import (
	"context"
	"fmt"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/externalnode"
)

// FetchRegisteredNode retrieves the backend node represented by a local joined-device registration.
func FetchRegisteredNode(
	ctx context.Context,
	nodeClients externalnode.NodeClientFactory,
	tokenProvider externalnode.TokenProvider,
	reg *DeviceRegistration,
) (*nodev1.ExternalNode, error) {
	client := nodeClients.NewNodeClient(tokenProvider, config.GlobalConfig.GetBrevPublicAPIURL())
	resp, err := client.GetNode(ctx, connect.NewRequest(&nodev1.GetNodeRequest{
		ExternalNodeId: reg.ExternalNodeID,
		OrganizationId: reg.OrgID,
	}))
	if err != nil {
		return nil, fmt.Errorf("error retrieving joined node: %w", err)
	}
	return registeredNodeFromResponse(resp)
}

func registeredNodeFromResponse(resp *connect.Response[nodev1.GetNodeResponse]) (*nodev1.ExternalNode, error) {
	if resp == nil || resp.Msg == nil || resp.Msg.GetExternalNode() == nil {
		return nil, fmt.Errorf(`registered node was not returned by Brev; run "brev leave" and "brev join" to repair membership`)
	}
	return resp.Msg.GetExternalNode(), nil
}
