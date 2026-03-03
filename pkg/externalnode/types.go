package externalnode

import (
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
)

// TokenProvider abstracts access token retrieval for authenticated RPC calls.
type TokenProvider interface {
	GetAccessToken() (string, error)
}

// PlatformChecker checks whether the current platform is supported.
type PlatformChecker interface {
	IsCompatible() bool
}

// NodeClientFactory creates ConnectRPC ExternalNodeService clients.
type NodeClientFactory interface {
	NewNodeClient(provider TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient
}

// FriendlyNetworkStatus returns a human-readable label for a NetworkMemberStatus.
func FriendlyNetworkStatus(s nodev1.NetworkMemberStatus) string {
	switch s {
	case nodev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_CONNECTED:
		return "Connected"
	case nodev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_DISCONNECTED:
		return "Disconnected"
	case nodev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_UNSPECIFIED:
		return "Registered"
	default:
		return "Unknown"
	}
}
