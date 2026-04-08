package shell

import (
	"errors"
	"strings"
	"testing"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
)

func strPtr(s string) *string { return &s }

// TestResolveExternalNodeSSH_BuildsCorrectInfo tests that the SSH info
// returned by ResolveNodeSSHEntry has the correct target, alias, and home path —
// the same values shellIntoExternalNode uses to build its SSH command.
func TestResolveExternalNodeSSH_BuildsCorrectInfo(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "My GPU Box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ec2-user"},
		},
		Ports: []*nodev1.Port{
			{
				Protocol:   nodev1.PortProtocol_PORT_PROTOCOL_SSH,
				PortNumber: 22,
				ServerPort: 41920,
				Hostname:   strPtr("10.0.0.5"),
			},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	// Build ExternalNodeSSHInfo the same way ResolveExternalNodeSSH does.
	info := &util.ExternalNodeSSHInfo{
		Node:      node,
		LinuxUser: entry.User,
		Hostname:  entry.Hostname,
		Port:      entry.Port,
	}

	// SSHTarget — used by runSSHWithPort
	if got := info.SSHTarget(); got != "ec2-user@10.0.0.5" {
		t.Errorf("SSHTarget() = %q, want %q", got, "ec2-user@10.0.0.5")
	}

	// SSHAlias — used by SSH config-based commands (open, copy, port-forward)
	if got := info.SSHAlias(); got != "my-gpu-box" {
		t.Errorf("SSHAlias() = %q, want %q", got, "my-gpu-box")
	}

	// HomePath — used by open to set the remote directory
	if got := info.HomePath(); got != "/home/ec2-user" {
		t.Errorf("HomePath() = %q, want %q", got, "/home/ec2-user")
	}

	// Port — shellIntoExternalNode passes this to runSSHWithPort
	if info.Port != 41920 {
		t.Errorf("Port = %d, want 41920", info.Port)
	}
}

// TestResolveExternalNodeSSH_NoAccess verifies that a node without SSH access
// for the given user returns nil, which shellIntoExternalNode would treat as an error.
func TestResolveExternalNodeSSH_NoAccess(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "locked-box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "other_user", LinuxUser: "root"},
		},
		Ports: []*nodev1.Port{
			{Protocol: nodev1.PortProtocol_PORT_PROTOCOL_SSH, ServerPort: 22, Hostname: strPtr("10.0.0.1")},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry != nil {
		t.Errorf("expected nil for user without access, got %+v", entry)
	}
}

// TestResolveExternalNodeSSH_NoSSHPort verifies that a node with no SSH port
// returns nil even when the user has access.
func TestResolveExternalNodeSSH_NoSSHPort(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "no-ssh",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu"},
		},
		Ports: []*nodev1.Port{
			{Protocol: nodev1.PortProtocol_PORT_PROTOCOL_TCP, ServerPort: 8080, Hostname: strPtr("10.0.0.1")},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry != nil {
		t.Errorf("expected nil for node without SSH port, got %+v", entry)
	}
}

func TestWrapSSHConfigRefreshError(t *testing.T) {
	rootErr := errors.New("permission denied")

	err := wrapSSHConfigRefreshError(rootErr)

	if !strings.Contains(err.Error(), "brev print-config") {
		t.Fatalf("expected error to mention brev print-config, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), "failed to refresh SSH config automatically") {
		t.Fatalf("expected refresh guidance in error, got %q", err.Error())
	}

	if !errors.Is(err, rootErr) {
		t.Fatalf("expected wrapped error to preserve original error")
	}
}
