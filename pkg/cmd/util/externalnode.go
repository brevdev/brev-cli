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
// for a given user from a node. Returns nil if the user has no SSH access or no
// resolvable port with a hostname. Uses the port matching the user's access PortId.
func ResolveNodeSSHEntry(userID string, node *nodev1.ExternalNode) *ssh.ExternalNodeSSHEntry {
	var access *nodev1.SSHAccess
	for _, a := range node.GetSshAccess() {
		if a.GetUserId() == userID {
			access = a
			break
		}
	}
	if access == nil || access.GetLinuxUser() == "" {
		return nil
	}

	port := resolvePortForSSHAccess(node, access)
	if port == nil || port.GetHostname() == "" {
		return nil
	}

	return &ssh.ExternalNodeSSHEntry{
		Alias:    ssh.SanitizeNodeName(node.GetName()),
		Hostname: port.GetHostname(),
		Port:     port.GetPortNumber(),
		User:     access.GetLinuxUser(),
	}
}

func resolvePortForSSHAccess(node *nodev1.ExternalNode, access *nodev1.SSHAccess) *nodev1.Port {
	portID := access.GetPortId()
	for _, p := range node.GetPorts() {
		if p.GetPortId() == portID {
			return p
		}
	}
	return nil
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
// by finding the current user's SSH access and the allocated port for that access.
func ResolveExternalNodeSSH(store ExternalNodeStore, node *nodev1.ExternalNode) (*ExternalNodeSSHInfo, error) {
	user, err := store.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	entry := ResolveNodeSSHEntry(user.ID, node)
	if entry == nil {
		return nil, classifyNodeSSHFailure(user, node)
	}

	return &ExternalNodeSSHInfo{
		Node:      node,
		LinuxUser: entry.User,
		Hostname:  entry.Hostname,
		Port:      entry.Port,
	}, nil
}

// classifyNodeSSHFailure returns an actionable error explaining why SSH cannot
// be resolved for the user on this node. The cases mirror ResolveNodeSSHEntry:
// the user may have no access grant, an access grant whose port is not yet
// allocated, or a port that has not yet reported its hostname. Each case has a
// distinct remedy, so we surface them separately rather than as one catch-all.
func classifyNodeSSHFailure(user *entity.User, node *nodev1.ExternalNode) error {
	nodeName := node.GetName()

	var access *nodev1.SSHAccess
	for _, a := range node.GetSshAccess() {
		if a.GetUserId() == user.ID {
			access = a
			break
		}
	}

	// Most common case (per the issue): the node is visible in `brev ls nodes`
	// because the user is in the org, but no SSH access has been granted.
	if access == nil || access.GetLinuxUser() == "" {
		who := user.Email
		if who == "" {
			who = user.ID
		}
		return breverrors.New(fmt.Sprintf(
			"you don't have SSH access to node %q.\n"+
				"Ask an org admin to grant you access, e.g.:\n"+
				"  brev grant-ssh --node %s --user %s",
			nodeName, nodeName, who))
	}

	port := resolvePortForSSHAccess(node, access)
	if port == nil {
		return breverrors.New(fmt.Sprintf(
			"SSH access to node %q is granted but its SSH port isn't allocated yet.\n"+
				"The node may still be connecting — try again shortly, or run 'brev refresh'.",
			nodeName))
	}

	// Access and port exist, but the port has no hostname yet.
	return breverrors.New(fmt.Sprintf(
		"SSH access to node %q is granted but the connection details aren't ready yet.\n"+
			"The node may still be connecting — try again shortly, or run 'brev refresh'.",
		nodeName))
}
