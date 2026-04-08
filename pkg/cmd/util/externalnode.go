package util

import (
	"context"
	"fmt"
	"strings"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/ssh"
)

// ExternalNodeStore is the minimal interface needed for external node lookup and SSH resolution.
type ExternalNodeStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetAccessToken() (string, error)
	GetCurrentUser() (*entity.User, error)
}

type WorkspaceOrNodeResolver interface {
	GetWorkspaceByNameOrIDErrStore
	ExternalNodeStore
}

// WorkspaceOrNode is returned by ResolveWorkspaceOrNode. Exactly one field is non-nil.
type WorkspaceOrNode struct {
	Workspace *entity.Workspace
	Node      *nodev1.ExternalNode
}

// ResolveWorkspaceOrNode looks up a workspace first; if not found, falls back to external nodes.
// The store must satisfy both GetWorkspaceByNameOrIDErrStore and ExternalNodeStore.
func ResolveWorkspaceOrNode(store WorkspaceOrNodeResolver, nameOrID string,
) (*WorkspaceOrNode, error) {
	workspace, wsErr := GetUserWorkspaceByNameOrIDErr(store, nameOrID)
	if wsErr == nil {
		return &WorkspaceOrNode{Workspace: workspace}, nil
	}
	node, nodeErr := FindExternalNode(store, nameOrID)
	if nodeErr != nil || node == nil {
		return nil, wsErr // return original workspace error
	}
	return &WorkspaceOrNode{Node: node}, nil
}

// ResolveWorkspaceOrNodeInOrg is like ResolveWorkspaceOrNode but looks up workspaces in a specific org.
func ResolveWorkspaceOrNodeInOrg(store WorkspaceOrNodeResolver, nameOrID string, orgID string,
) (*WorkspaceOrNode, error) {
	workspace, wsErr := GetUserWorkspaceByNameOrIDErrInOrg(store, nameOrID, orgID)
	if wsErr == nil {
		return &WorkspaceOrNode{Workspace: workspace}, nil
	}
	node, nodeErr := FindExternalNode(store, nameOrID)
	if nodeErr != nil || node == nil {
		return nil, wsErr // return original workspace error
	}
	return &WorkspaceOrNode{Node: node}, nil
}

// ExternalNodeSSHInfo holds resolved SSH connection details for an external node.
type ExternalNodeSSHInfo struct {
	Node      *nodev1.ExternalNode
	LinuxUser string
	Hostname  string
	Port      int32
}

// SSHTarget returns the "user@host" string for direct SSH.
func (info *ExternalNodeSSHInfo) SSHTarget() string {
	return fmt.Sprintf("%s@%s", info.LinuxUser, info.Hostname)
}

// SSHAlias returns a sanitized node name suitable for use as an SSH config Host alias.
func (info *ExternalNodeSSHInfo) SSHAlias() string {
	return ssh.SanitizeNodeName(info.Node.GetName())
}

// HomePath returns the home directory path for the linux user.
func (info *ExternalNodeSSHInfo) HomePath() string {
	return fmt.Sprintf("/home/%s", info.LinuxUser)
}

// ResolveNodeSSHEntry is a pure data function that extracts the SSH config entry
// for a given user from a node. Returns nil if the user has no access or the node
// has no SSH port. This is the single source of truth for node→SSHEntry conversion,
// used by both ResolveExternalNodeSSH (for commands) and refresh (for SSH config generation).
func ResolveNodeSSHEntry(userID string, node *nodev1.ExternalNode) *ssh.ExternalNodeSSHEntry {
	var linuxUser string
	for _, access := range node.GetSshAccess() {
		if access.GetUserId() == userID {
			linuxUser = access.GetLinuxUser()
			break
		}
	}
	if linuxUser == "" {
		return nil
	}

	var sshPort *nodev1.Port
	for _, p := range node.GetPorts() {
		if p.GetProtocol() == nodev1.PortProtocol_PORT_PROTOCOL_SSH {
			sshPort = p
			break
		}
	}
	if sshPort == nil || sshPort.GetHostname() == "" {
		return nil
	}

	return &ssh.ExternalNodeSSHEntry{
		Alias:    ssh.SanitizeNodeName(node.GetName()),
		Hostname: sshPort.GetHostname(),
		Port:     sshPort.GetServerPort(),
		User:     linuxUser,
	}
}

// OpenPort calls the OpenPort RPC to open a port on an external node via netbird.
// This must be called before attempting to connect to a non-SSH port on a node.
func OpenPort(store ExternalNodeStore, nodeID string, portNumber int32, protocol nodev1.PortProtocol) (*nodev1.Port, error) {
	client := register.NewNodeServiceClient(store, config.GlobalConfig.GetBrevPublicAPIURL())
	resp, err := client.OpenPort(context.Background(), connect.NewRequest(&nodev1.OpenPortRequest{
		ExternalNodeId: nodeID,
		Protocol:       protocol,
		PortNumber:     portNumber,
	}))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return resp.Msg.GetPort(), nil
}

// FindExternalNode searches for an external node by name in the user's active organization.
// Returns (nil, nil) if no matching node is found.
func FindExternalNode(store ExternalNodeStore, name string) (*nodev1.ExternalNode, error) {
	org, err := store.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	client := register.NewNodeServiceClient(store, config.GlobalConfig.GetBrevPublicAPIURL())
	resp, err := client.ListNodes(context.Background(), connect.NewRequest(&nodev1.ListNodesRequest{
		OrganizationId: org.ID,
	}))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	for _, node := range resp.Msg.GetItems() {
		if strings.EqualFold(node.GetName(), name) {
			return node, nil
		}
	}
	return nil, nil
}

// ResolveExternalNodeSSH resolves the SSH connection details for an external node
// by finding the current user's SSH access and the node's SSH port.
func ResolveExternalNodeSSH(store ExternalNodeStore, node *nodev1.ExternalNode) (*ExternalNodeSSHInfo, error) {
	user, err := store.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	entry := ResolveNodeSSHEntry(user.ID, node)
	if entry == nil {
		return nil, breverrors.New(fmt.Sprintf("cannot resolve SSH for node %q — no access, no SSH port, or no hostname", node.GetName()))
	}

	return &ExternalNodeSSHInfo{
		Node:      node,
		LinuxUser: entry.User,
		Hostname:  entry.Hostname,
		Port:      entry.Port,
	}, nil
}
