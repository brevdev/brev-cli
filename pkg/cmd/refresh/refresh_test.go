package refresh

import (
	"testing"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
)

func strPtr(s string) *string { return &s }

type identityCountingStore struct {
	RefreshStore
	org         *entity.Organization
	user        *entity.User
	activeOrgs  int
	currentUser int
	workspaces  []entity.Workspace

	forIdentity []string
	resolving   int
}

func (s *identityCountingStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	s.activeOrgs++
	return s.org, nil
}

func (s *identityCountingStore) GetCurrentUser() (*entity.User, error) {
	s.currentUser++
	return s.user, nil
}

func (s *identityCountingStore) GetContextWorkspaces() ([]entity.Workspace, error) {
	s.resolving++
	return s.workspaces, nil
}

func (s *identityCountingStore) DevPlane() *store.DevPlaneClient {
	return store.NewDevPlaneClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
}

func (s *identityCountingStore) GetContextWorkspacesFor(orgID, userID string) ([]entity.Workspace, error) {
	s.forIdentity = append(s.forIdentity, orgID+"/"+userID)
	return s.workspaces, nil
}

func TestResolveRefreshIdentity_ResolvesOrgAndUserOnce(t *testing.T) {
	s := &identityCountingStore{
		org:  &entity.Organization{ID: "org_1"},
		user: &entity.User{ID: "user_1"},
	}

	identity := resolveRefreshIdentity(s)

	require.NotNil(t, identity.org)
	require.NotNil(t, identity.user)
	assert.Equal(t, "org_1", identity.org.ID)
	assert.Equal(t, "user_1", identity.user.ID)
	assert.Equal(t, 1, s.activeOrgs)
	assert.Equal(t, 1, s.currentUser)
}

func TestWorkspaceSSHStore_ReusesResolvedIdentity(t *testing.T) {
	user := &entity.User{ID: "user_1"}
	org := &entity.Organization{ID: "org_1"}
	s := &identityCountingStore{
		org:        org,
		user:       user,
		workspaces: []entity.Workspace{{ID: "ws_1", Status: entity.Stopped}},
	}

	got, err := workspaceSSHStore{RefreshStore: s, org: org, user: user}.GetContextWorkspaces()

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 0, s.currentUser, "the resolved user must be reused, not fetched again")
	assert.Equal(t, 0, s.activeOrgs, "the resolved org must be reused, not fetched again")
	assert.Equal(t, []string{"org_1/user_1"}, s.forIdentity, "listing must use the refresh identity")
	assert.Equal(t, 0, s.resolving, "listing must not fall back to re-resolving org and user")
}

func TestWorkspaceSSHStore_FallsBackWhenIdentityMissing(t *testing.T) {
	s := &identityCountingStore{
		user:       &entity.User{ID: "user_1"},
		workspaces: []entity.Workspace{{ID: "ws_1", Status: entity.Stopped}},
	}

	got, err := workspaceSSHStore{RefreshStore: s}.GetContextWorkspaces()

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 1, s.resolving, "expected the store's own resolution to be used")
	assert.Equal(t, 1, s.currentUser, "the enrichment falls back to resolving the user")
}

func TestGetExternalNodeSSHEntries_NoIdentitySkipsLookups(t *testing.T) {
	s := &identityCountingStore{}

	assert.Nil(t, getExternalNodeSSHEntries(s, refreshIdentity{}))
	assert.Equal(t, 0, s.activeOrgs)
	assert.Equal(t, 0, s.currentUser)
}

func TestResolveNodeSSHEntry_HappyPath(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "My GPU Box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ec2-user", PortId: "port_1"},
		},
		Ports: []*nodev1.Port{
			{
				PortId:     "port_1",
				Protocol:   nodev1.PortProtocol_PORT_PROTOCOL_TCP,
				PortNumber: 41920,
				ServerPort: 22,
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
			{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_1"},
		},
		Ports: []*nodev1.Port{
			{
				PortId:     "port_1",
				Protocol:   nodev1.PortProtocol_PORT_PROTOCOL_TCP,
				PortNumber: 51234, // netbird-assigned port — correct
				ServerPort: 22,    // well-known port — NOT what we should connect to
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
			{PortId: "port_1", Protocol: nodev1.PortProtocol_PORT_PROTOCOL_TCP, ServerPort: 22, Hostname: strPtr("h")},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry != nil {
		t.Errorf("expected nil for no access, got %+v", entry)
	}
}

func TestResolveNodeSSHEntry_UsesAccessPortNotProtocol(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_tcp"},
		},
		Ports: []*nodev1.Port{
			{PortId: "port_tcp", Protocol: nodev1.PortProtocol_PORT_PROTOCOL_TCP, PortNumber: 51234, ServerPort: 22, Hostname: strPtr("h")},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry == nil {
		t.Fatal("expected entry for TCP port with access")
	}
	if entry.Port != 51234 {
		t.Errorf("expected port 51234, got %d", entry.Port)
	}
}

func TestResolveNodeSSHEntry_SkipsEmptyHostname(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_1"},
		},
		Ports: []*nodev1.Port{
			{PortId: "port_1", Protocol: nodev1.PortProtocol_PORT_PROTOCOL_TCP, ServerPort: 22},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry != nil {
		t.Errorf("expected nil for empty hostname, got %+v", entry)
	}
}

func TestResolveNodeSSHEntry_SkipsWhenPortIDMissing(t *testing.T) {
	node := &nodev1.ExternalNode{
		Name: "box",
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "user_1", LinuxUser: "ubuntu"},
		},
		Ports: []*nodev1.Port{
			{PortId: "port_a", PortNumber: 41000, ServerPort: 22, Hostname: strPtr("10.0.0.1")},
		},
	}

	entry := util.ResolveNodeSSHEntry("user_1", node)
	if entry != nil {
		t.Errorf("expected nil without PortId on access, got %+v", entry)
	}
}

func TestResolveNodeSSHEntry_MultipleNodes(t *testing.T) {
	nodes := []*nodev1.ExternalNode{
		{
			Name: "Node A",
			SshAccess: []*nodev1.SSHAccess{
				{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_a"},
			},
			Ports: []*nodev1.Port{
				{PortId: "port_a", PortNumber: 41000, ServerPort: 22, Hostname: strPtr("10.0.0.1")},
			},
		},
		{
			Name: "Node B",
			SshAccess: []*nodev1.SSHAccess{
				{UserId: "other_user", LinuxUser: "root", PortId: "port_b"},
			},
			Ports: []*nodev1.Port{
				{PortId: "port_b", PortNumber: 42000, ServerPort: 22, Hostname: strPtr("10.0.0.2")},
			},
		},
		{
			Name: "Node C",
			SshAccess: []*nodev1.SSHAccess{
				{UserId: "user_1", LinuxUser: "admin", PortId: "port_c"},
			},
			Ports: []*nodev1.Port{
				{PortId: "port_c", PortNumber: 43000, ServerPort: 22, Hostname: strPtr("10.0.0.3")},
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
