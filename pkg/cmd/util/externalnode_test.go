package util

import (
	"fmt"
	"strings"
	"testing"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
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

func makeTestNode(name, userID, linuxUser, hostname string, portNumber int32) *nodev1.ExternalNode {
	portID := "port_test"
	return &nodev1.ExternalNode{
		ExternalNodeId: "unode_test",
		Name:           name,
		SshAccess: []*nodev1.SSHAccess{
			{UserId: userID, LinuxUser: linuxUser, PortId: portID},
		},
		Ports: []*nodev1.Port{
			{
				PortId:     portID,
				Protocol:   nodev1.PortProtocol_PORT_PROTOCOL_TCP,
				PortNumber: portNumber,
				ServerPort: 22,
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
			{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_1"},
		},
		Ports: []*nodev1.Port{
			{
				PortId:     "port_1",
				Protocol:   nodev1.PortProtocol_PORT_PROTOCOL_TCP,
				PortNumber: 41920, // netbird-assigned port — this is correct
				ServerPort: 22,    // well-known port — NOT what we connect to
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
		user: &entity.User{ID: "user_1", Email: "me@example.com"},
	}
	node := makeTestNode("box", "other_user", "ubuntu", "10.0.0.5", 22)

	_, err := ResolveExternalNodeSSH(store, node)
	if err == nil {
		t.Fatal("expected error for no SSH access")
	}
	// The message should tell the user they lack access and how to get it,
	// not the old confusing "no access, no port, or no hostname".
	if !strings.Contains(err.Error(), "don't have SSH access") {
		t.Errorf("expected a 'no access' message, got: %v", err)
	}
	// It should point the user at someone who can grant access, not an org admin.
	if !strings.Contains(err.Error(), "someone with SSH access to this node") {
		t.Errorf("expected the message to ask someone with SSH access, got: %v", err)
	}
	if strings.Contains(err.Error(), "org admin") {
		t.Errorf("did not expect the message to reference an org admin, got: %v", err)
	}
	if !strings.Contains(err.Error(), "brev grant-ssh") {
		t.Errorf("expected the message to suggest 'brev grant-ssh', got: %v", err)
	}
	if !strings.Contains(err.Error(), "me@example.com") {
		t.Errorf("expected the message to reference the user's email, got: %v", err)
	}
	// It must be a ValidationError so the top-level handler prints it cleanly
	// (no stack trace, no Sentry report) even in dev builds.
	var valErr breverrors.ValidationError
	if !breverrors.As(err, &valErr) {
		t.Errorf("expected a ValidationError, got %T: %v", err, err)
	}
}

func TestResolveExternalNodeSSH_NoPortAllocated(t *testing.T) {
	store := &mockExternalNodeStore{
		user: &entity.User{ID: "user_1"},
	}
	// User has an access grant, but no port matches its PortId.
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_missing"},
		},
		Ports: []*nodev1.Port{},
	}

	_, err := ResolveExternalNodeSSH(store, node)
	if err == nil {
		t.Fatal("expected error when access exists but no port is allocated")
	}
	if !strings.Contains(err.Error(), "is granted") {
		t.Errorf("expected a 'granted but not ready' message, got: %v", err)
	}
}

func TestResolveExternalNodeSSH_UsesAccessPortNotProtocol(t *testing.T) {
	store := &mockExternalNodeStore{
		user: &entity.User{ID: "user_1"},
	}
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_tcp"},
		},
		Ports: []*nodev1.Port{
			{PortId: "port_tcp", Protocol: nodev1.PortProtocol_PORT_PROTOCOL_TCP, PortNumber: 51234, ServerPort: 22, Hostname: strPtr("h")},
		},
	}

	info, err := ResolveExternalNodeSSH(store, node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Port != 51234 {
		t.Errorf("expected port 51234, got %d", info.Port)
	}
}

func TestResolveExternalNodeSSH_EmptyHostname(t *testing.T) {
	store := &mockExternalNodeStore{
		user: &entity.User{ID: "user_1"},
	}
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_1"},
		},
		Ports: []*nodev1.Port{
			{PortId: "port_1", Protocol: nodev1.PortProtocol_PORT_PROTOCOL_TCP, ServerPort: 22},
		},
	}

	_, err := ResolveExternalNodeSSH(store, node)
	if err == nil {
		t.Fatal("expected error for empty hostname")
	}
	if !strings.Contains(err.Error(), "is granted") {
		t.Errorf("expected a 'granted but not ready' message, got: %v", err)
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
