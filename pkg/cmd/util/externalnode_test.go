package util

import (
	"fmt"
	"testing"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/entity"
)

// mockExternalNodeStore satisfies ExternalNodeStore for unit tests that
// only exercise ResolveExternalNodeSSH (no RPC calls).
type mockExternalNodeStore struct {
	user *entity.User
	org  *entity.Organization
	err  error
}

func (m *mockExternalNodeStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return m.org, nil
}

func (m *mockExternalNodeStore) GetAccessToken() (string, error) { return "tok", nil }

func (m *mockExternalNodeStore) GetCurrentUser() (*entity.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

func strPtr(s string) *string { return &s }

func makeTestNode(name, userID, linuxUser, hostname string, serverPort int32) *nodev1.ExternalNode {
	return &nodev1.ExternalNode{
		ExternalNodeId: "unode_test",
		Name:           name,
		SshAccess: []*nodev1.SSHAccess{
			{UserId: userID, LinuxUser: linuxUser},
		},
		Ports: []*nodev1.Port{
			{
				Protocol:   nodev1.PortProtocol_PORT_PROTOCOL_SSH,
				PortNumber: 22,
				ServerPort: serverPort,
				Hostname:   &hostname,
			},
		},
	}
}

func TestResolveExternalNodeSSH_HappyPath(t *testing.T) {
	store := &mockExternalNodeStore{
		user: &entity.User{ID: "user_1"},
	}
	node := makeTestNode("My GPU Box", "user_1", "ec2-user", "10.0.0.5", 51234)

	info, err := ResolveExternalNodeSSH(store, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.LinuxUser != "ec2-user" {
		t.Errorf("expected ec2-user, got %s", info.LinuxUser)
	}
	if info.Hostname != "10.0.0.5" {
		t.Errorf("expected 10.0.0.5, got %s", info.Hostname)
	}
	if info.Port != 51234 {
		t.Errorf("expected port 51234, got %d", info.Port)
	}
}

func TestResolveExternalNodeSSH_UsesServerPortNotPortNumber(t *testing.T) {
	store := &mockExternalNodeStore{
		user: &entity.User{ID: "user_1"},
	}
	node := &nodev1.ExternalNode{
		Name: "test-node",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu"},
		},
		Ports: []*nodev1.Port{
			{
				Protocol:   nodev1.PortProtocol_PORT_PROTOCOL_SSH,
				PortNumber: 22,    // well-known port — NOT what we connect to
				ServerPort: 41920, // netbird-assigned port — this is correct
				Hostname:   strPtr("gateway.example.com"),
			},
		},
	}

	info, err := ResolveExternalNodeSSH(store, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Port != 41920 {
		t.Errorf("expected ServerPort 41920, got %d (should not use PortNumber 22)", info.Port)
	}
}

func TestResolveExternalNodeSSH_NoAccess(t *testing.T) {
	store := &mockExternalNodeStore{
		user: &entity.User{ID: "user_1"},
	}
	node := makeTestNode("box", "other_user", "ubuntu", "10.0.0.5", 22)

	_, err := ResolveExternalNodeSSH(store, node)
	if err == nil {
		t.Fatal("expected error for no SSH access")
	}
}

func TestResolveExternalNodeSSH_NoSSHPort(t *testing.T) {
	store := &mockExternalNodeStore{
		user: &entity.User{ID: "user_1"},
	}
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu"},
		},
		Ports: []*nodev1.Port{
			{Protocol: nodev1.PortProtocol_PORT_PROTOCOL_TCP, Hostname: strPtr("h")},
		},
	}

	_, err := ResolveExternalNodeSSH(store, node)
	if err == nil {
		t.Fatal("expected error for no SSH port")
	}
}

func TestResolveExternalNodeSSH_EmptyHostname(t *testing.T) {
	store := &mockExternalNodeStore{
		user: &entity.User{ID: "user_1"},
	}
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu"},
		},
		Ports: []*nodev1.Port{
			{Protocol: nodev1.PortProtocol_PORT_PROTOCOL_SSH, ServerPort: 22},
		},
	}

	_, err := ResolveExternalNodeSSH(store, node)
	if err == nil {
		t.Fatal("expected error for empty hostname")
	}
}

func TestResolveExternalNodeSSH_GetUserError(t *testing.T) {
	store := &mockExternalNodeStore{
		err: fmt.Errorf("auth failed"),
	}
	node := makeTestNode("box", "user_1", "ubuntu", "h", 22)

	_, err := ResolveExternalNodeSSH(store, node)
	if err == nil {
		t.Fatal("expected error when GetCurrentUser fails")
	}
}

func TestExternalNodeSSHInfo_SSHTarget(t *testing.T) {
	info := &ExternalNodeSSHInfo{LinuxUser: "ec2-user", Hostname: "10.0.0.5"}
	if got := info.SSHTarget(); got != "ec2-user@10.0.0.5" {
		t.Errorf("expected ec2-user@10.0.0.5, got %s", got)
	}
}

func TestExternalNodeSSHInfo_SSHAlias(t *testing.T) {
	info := &ExternalNodeSSHInfo{
		Node: &nodev1.ExternalNode{Name: "My GPU Box"},
	}
	if got := info.SSHAlias(); got != "my-gpu-box" {
		t.Errorf("expected my-gpu-box, got %s", got)
	}
}

func TestExternalNodeSSHInfo_HomePath(t *testing.T) {
	info := &ExternalNodeSSHInfo{LinuxUser: "ec2-user"}
	if got := info.HomePath(); got != "/home/ec2-user" {
		t.Errorf("expected /home/ec2-user, got %s", got)
	}
}
