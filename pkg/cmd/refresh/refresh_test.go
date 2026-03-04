package refresh

import (
	"testing"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
)

func strPtr(s string) *string { return &s }

func TestResolveNodeSSHEntry_HappyPath(t *testing.T) {
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
	if entry.Alias != "my-gpu-box" {
		t.Errorf("expected alias my-gpu-box, got %s", entry.Alias)
	}
	if entry.Hostname != "10.0.0.5" {
		t.Errorf("expected hostname 10.0.0.5, got %s", entry.Hostname)
	}
	if entry.Port != 41920 {
		t.Errorf("expected port 41920 (ServerPort), got %d", entry.Port)
	}
	if entry.User != "ec2-user" {
		t.Errorf("expected user ec2-user, got %s", entry.User)
	}
}

func TestResolveNodeSSHEntry_UsesServerPortNotPortNumber(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "test-node",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu"},
		},
		Ports: []*nodev1.Port{
			{
				Protocol:   nodev1.PortProtocol_PORT_PROTOCOL_SSH,
				PortNumber: 22,    // well-known port — NOT what we should connect to
				ServerPort: 51234, // netbird-assigned port — correct
				Hostname:   strPtr("gateway.example.com"),
			},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.Port != 51234 {
		t.Errorf("expected ServerPort 51234, got %d (should not use PortNumber 22)", entry.Port)
	}
}

func TestResolveNodeSSHEntry_SkipsNoAccess(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "other_user", LinuxUser: "ubuntu"},
		},
		Ports: []*nodev1.Port{
			{Protocol: nodev1.PortProtocol_PORT_PROTOCOL_SSH, ServerPort: 22, Hostname: strPtr("h")},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry != nil {
		t.Errorf("expected nil for no access, got %+v", entry)
	}
}

func TestResolveNodeSSHEntry_SkipsNoSSHPort(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu"},
		},
		Ports: []*nodev1.Port{
			{Protocol: nodev1.PortProtocol_PORT_PROTOCOL_TCP, ServerPort: 8080, Hostname: strPtr("h")},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry != nil {
		t.Errorf("expected nil for no SSH port, got %+v", entry)
	}
}

func TestResolveNodeSSHEntry_SkipsEmptyHostname(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu"},
		},
		Ports: []*nodev1.Port{
			{Protocol: nodev1.PortProtocol_PORT_PROTOCOL_SSH, ServerPort: 22},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry != nil {
		t.Errorf("expected nil for empty hostname, got %+v", entry)
	}
}

func TestResolveNodeSSHEntry_MultipleNodes(t *testing.T) {
	nodes := []*nodev1.ExternalNode{
		{
			Name: "Node A",
			SshAccess: []*nodev1.SSHAccess{
				{UserId: "user_1", LinuxUser: "ubuntu"},
			},
			Ports: []*nodev1.Port{
				{Protocol: nodev1.PortProtocol_PORT_PROTOCOL_SSH, ServerPort: 41000, Hostname: strPtr("10.0.0.1")},
			},
		},
		{
			Name: "Node B",
			SshAccess: []*nodev1.SSHAccess{
				{UserId: "other_user", LinuxUser: "root"},
			},
			Ports: []*nodev1.Port{
				{Protocol: nodev1.PortProtocol_PORT_PROTOCOL_SSH, ServerPort: 42000, Hostname: strPtr("10.0.0.2")},
			},
		},
		{
			Name: "Node C",
			SshAccess: []*nodev1.SSHAccess{
				{UserId: "user_1", LinuxUser: "admin"},
			},
			Ports: []*nodev1.Port{
				{Protocol: nodev1.PortProtocol_PORT_PROTOCOL_SSH, ServerPort: 43000, Hostname: strPtr("10.0.0.3")},
			},
		},
	}

	var entries []string
	for _, node := range nodes {
		entry := util.ResolveNodeSSHEntry("user_1", node)
		if entry != nil {
			entries = append(entries, entry.Alias)
		}
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (skipping Node B), got %d", len(entries))
	}
	if entries[0] != "node-a" {
		t.Errorf("expected alias node-a, got %s", entries[0])
	}
	if entries[1] != "node-c" {
		t.Errorf("expected alias node-c, got %s", entries[1])
	}
}
