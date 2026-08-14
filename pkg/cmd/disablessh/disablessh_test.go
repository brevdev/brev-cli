package disablessh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type disableSSHTestStore struct {
	currentUser      *entity.User
	currentUserErr   error
	currentUserCalls int
}

func (s *disableSSHTestStore) GetCurrentUser() (*entity.User, error) {
	s.currentUserCalls++
	return s.currentUser, s.currentUserErr
}

func (*disableSSHTestStore) GetAccessToken() (string, error) {
	return "token", nil
}

type disableSSHTestRegistrationStore struct {
	exists      bool
	existsErr   error
	loadErr     error
	reg         *register.DeviceRegistration
	saveCalls   int
	deleteCalls int
}

func (s *disableSSHTestRegistrationStore) Exists() (bool, error) {
	return s.exists, s.existsErr
}

func (s *disableSSHTestRegistrationStore) Load() (*register.DeviceRegistration, error) {
	return s.reg, s.loadErr
}

func (s *disableSSHTestRegistrationStore) Save(*register.DeviceRegistration) error {
	s.saveCalls++
	return nil
}

func (s *disableSSHTestRegistrationStore) Delete() error {
	s.deleteCalls++
	return nil
}

type disableSSHTestConfirmer struct {
	answer bool
	calls  int
}

func (c *disableSSHTestConfirmer) ConfirmYesNo(string) bool {
	c.calls++
	return c.answer
}

type disableSSHRecordingClient struct {
	nodev1connect.ExternalNodeServiceClient

	node           *nodev1.ExternalNode
	getErr         error
	getNodeCalls   int
	revokeErrors   map[int]error
	revokeRequests []*nodev1.RevokeNodeSSHAccessRequest

	addNodeCalls    int
	removeNodeCalls int
	closePortCalls  int
}

func (c *disableSSHRecordingClient) GetNode(_ context.Context, req *connect.Request[nodev1.GetNodeRequest]) (*connect.Response[nodev1.GetNodeResponse], error) {
	c.getNodeCalls++
	if c.getErr != nil {
		return nil, c.getErr
	}
	if req.Msg.GetExternalNodeId() != "node_123" || req.Msg.GetOrganizationId() != "org_123" {
		return nil, fmt.Errorf("unexpected GetNode request: %+v", req.Msg)
	}
	return connect.NewResponse(&nodev1.GetNodeResponse{ExternalNode: c.node}), nil
}

func (c *disableSSHRecordingClient) RevokeNodeSSHAccess(_ context.Context, req *connect.Request[nodev1.RevokeNodeSSHAccessRequest]) (*connect.Response[nodev1.RevokeNodeSSHAccessResponse], error) {
	callIndex := len(c.revokeRequests)
	c.revokeRequests = append(c.revokeRequests, cloneRevokeRequest(req.Msg))
	if err := c.revokeErrors[callIndex]; err != nil {
		return nil, err
	}
	return connect.NewResponse(&nodev1.RevokeNodeSSHAccessResponse{}), nil
}

func (c *disableSSHRecordingClient) AddNode(context.Context, *connect.Request[nodev1.AddNodeRequest]) (*connect.Response[nodev1.AddNodeResponse], error) {
	c.addNodeCalls++
	return connect.NewResponse(&nodev1.AddNodeResponse{}), nil
}

func (c *disableSSHRecordingClient) RemoveNode(context.Context, *connect.Request[nodev1.RemoveNodeRequest]) (*connect.Response[nodev1.RemoveNodeResponse], error) {
	c.removeNodeCalls++
	return connect.NewResponse(&nodev1.RemoveNodeResponse{}), nil
}

func (c *disableSSHRecordingClient) ClosePort(context.Context, *connect.Request[nodev1.ClosePortRequest]) (*connect.Response[nodev1.ClosePortResponse], error) {
	c.closePortCalls++
	return connect.NewResponse(&nodev1.ClosePortResponse{}), nil
}

type disableSSHTestNodeClientFactory struct {
	client nodev1connect.ExternalNodeServiceClient
}

func (f disableSSHTestNodeClientFactory) NewNodeClient(externalnode.TokenProvider, string) nodev1connect.ExternalNodeServiceClient {
	return f.client
}

type disableSSHTestHarness struct {
	store         *disableSSHTestStore
	registrations *disableSSHTestRegistrationStore
	confirmer     *disableSSHTestConfirmer
	client        *disableSSHRecordingClient
	deps          disableSSHDeps
}

func newDisableSSHTestHarness(accesses ...*nodev1.SSHAccess) *disableSSHTestHarness {
	h := &disableSSHTestHarness{
		store: &disableSSHTestStore{currentUser: &entity.User{ID: "user_current"}},
		registrations: &disableSSHTestRegistrationStore{
			exists: true,
			reg: &register.DeviceRegistration{
				ExternalNodeID: "node_123",
				DisplayName:    "owned-node",
				OrgID:          "org_123",
				OrgName:        "owned-org",
			},
		},
		confirmer: &disableSSHTestConfirmer{answer: true},
		client: &disableSSHRecordingClient{
			node: &nodev1.ExternalNode{
				ExternalNodeId: "node_123",
				Name:           "owned-node",
				SshAccess:      accesses,
			},
			revokeErrors: make(map[int]error),
		},
	}
	h.deps = disableSSHDeps{
		confirmer:         h.confirmer,
		nodeClients:       disableSSHTestNodeClientFactory{client: h.client},
		registrationStore: h.registrations,
	}
	return h
}

func (h *disableSSHTestHarness) run(t *testing.T, skipConfirm bool) (stdout string, stderr string, err error) {
	t.Helper()
	var warnings bytes.Buffer
	stdout, err = captureDisableSSHStdout(t, func(term *terminal.Terminal) error {
		return runDisableSSH(context.Background(), term, &warnings, h.store, h.deps, skipConfirm)
	})
	return stdout, warnings.String(), err
}

func TestNewCmdDisableSSH_CommandSurface(t *testing.T) {
	cmd := NewCmdDisableSSH(terminal.New(), &disableSSHTestStore{})
	require.Equal(t, "disable-ssh", cmd.Use)
	require.Equal(t, "Revoke all Brev SSH access grants on this node", cmd.Short)
	require.NotNil(t, cmd.Args)
	require.Contains(t, cmd.Annotations, "configuration")
	require.Empty(t, cmd.Aliases)
	require.NotNil(t, cmd.Flags().Lookup("approve"))
}

func TestNewCmdDisableSSH_RejectsArguments(t *testing.T) {
	h := newDisableSSHTestHarness()
	cmd := newCmdDisableSSH(terminal.New(), h.store, h.deps)
	cmd.SetArgs([]string{"unexpected"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	require.Error(t, err)
	require.Empty(t, h.client.revokeRequests)
}

func TestRunDisableSSH_MissingRegistrationDoesNotAuthenticateOrCallRPC(t *testing.T) {
	h := newDisableSSHTestHarness()
	h.registrations.exists = false

	_, _, err := h.run(t, false)
	require.EqualError(t, err, `This machine has not joined a Brev network; run "brev join" first.`)
	require.Zero(t, h.store.currentUserCalls)
	require.Zero(t, h.client.getNodeCalls)
	require.Empty(t, h.client.revokeRequests)
}

func TestRunDisableSSH_RequiresCurrentUserIDBeforeLoadingGrants(t *testing.T) {
	authErr := errors.New("authentication failed")
	tests := []struct {
		name        string
		currentUser *entity.User
		currentErr  error
	}{
		{name: "authentication failure", currentErr: authErr},
		{name: "missing user"},
		{name: "missing user ID", currentUser: &entity.User{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newDisableSSHTestHarness(testSSHAccess("user_current", "ubuntu", "port_1"))
			h.store.currentUser = tt.currentUser
			h.store.currentUserErr = tt.currentErr

			_, _, err := h.run(t, true)
			require.Error(t, err)
			if tt.currentErr != nil {
				require.ErrorIs(t, err, tt.currentErr)
			} else {
				require.Contains(t, err.Error(), "missing user ID")
			}
			require.Zero(t, h.client.getNodeCalls)
			require.Zero(t, h.confirmer.calls)
			require.Empty(t, h.client.revokeRequests)
		})
	}
}

func TestRunDisableSSH_NoGrantsReportsAlreadyDisabledWithoutPrompting(t *testing.T) {
	h := newDisableSSHTestHarness()

	stdout, stderr, err := h.run(t, false)
	require.NoError(t, err)
	require.Contains(t, stdout, "No SSH access grants to revoke.")
	require.NotContains(t, stdout, "keys removed")
	require.Empty(t, stderr)
	require.Zero(t, h.confirmer.calls)
	require.Empty(t, h.client.revokeRequests)
}

func TestRunDisableSSH_CancelDoesNotRevoke(t *testing.T) {
	h := newDisableSSHTestHarness(testSSHAccess("user_collaborator", "ubuntu", "port_1"))
	h.confirmer.answer = false

	stdout, stderr, err := h.run(t, false)
	require.NoError(t, err)
	require.Contains(t, stdout, "Disable SSH canceled.")
	require.Contains(t, stderr, "node-wide operation")
	require.Equal(t, 1, h.confirmer.calls)
	require.Empty(t, h.client.revokeRequests)
}

func TestRunDisableSSH_ApproveRevokesWithoutPrompting(t *testing.T) {
	h := newDisableSSHTestHarness(testSSHAccess("user_collaborator", "ubuntu", "port_1"))

	stdout, stderr, err := h.run(t, true)
	require.NoError(t, err)
	require.Zero(t, h.confirmer.calls)
	require.Contains(t, stderr, "active SSH sessions are not forcibly terminated")
	require.Contains(t, stdout, "SSH access disabled. Grants revoked: 1.")
	require.Len(t, h.client.revokeRequests, 1)
}

func TestRunDisableSSH_RevokesEveryExactTupleWithCurrentUsersAccessLast(t *testing.T) {
	h := newDisableSSHTestHarness(
		testSSHAccess("user_current", "ubuntu", "port_self_1"),
		testSSHAccess("user_collaborator_1", "alice", "port_collaborator_1"),
		nil,
		testSSHAccess("user_current", "root", "port_self_2"),
		testSSHAccess("user_collaborator_2", "carol", "port_collaborator_2"),
	)

	_, _, err := h.run(t, true)
	require.NoError(t, err)
	require.Equal(t, []*nodev1.RevokeNodeSSHAccessRequest{
		{ExternalNodeId: "node_123", UserId: "user_collaborator_1", LinuxUser: "alice", PortId: "port_collaborator_1"},
		{ExternalNodeId: "node_123", UserId: "user_collaborator_2", LinuxUser: "carol", PortId: "port_collaborator_2"},
		{ExternalNodeId: "node_123", UserId: "user_current", LinuxUser: "ubuntu", PortId: "port_self_1"},
		{ExternalNodeId: "node_123", UserId: "user_current", LinuxUser: "root", PortId: "port_self_2"},
	}, h.client.revokeRequests)
}

func TestRunDisableSSH_ContinuesAfterFailuresAndReturnsEveryCause(t *testing.T) {
	firstErr := errors.New("first revoke failed")
	secondErr := errors.New("second revoke failed")
	h := newDisableSSHTestHarness(
		testSSHAccess("user_1", "ubuntu", "port_1"),
		testSSHAccess("user_2", "alice", "port_2"),
		testSSHAccess("user_current", "root", "port_self"),
	)
	h.client.revokeErrors[0] = firstErr
	h.client.revokeErrors[1] = secondErr

	_, _, err := h.run(t, true)
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, secondErr)
	require.Contains(t, err.Error(), "failed to revoke one or more SSH access grants")
	for _, text := range []string{"user_1", "ubuntu", "port_1", "user_2", "alice", "port_2"} {
		require.Contains(t, err.Error(), text)
	}
	require.Len(t, h.client.revokeRequests, 3)
}

func TestRunDisableSSH_DoesNotChangeMembershipPortsOrRegistration(t *testing.T) {
	h := newDisableSSHTestHarness(testSSHAccess("user_collaborator", "ubuntu", "port_1"))

	_, _, err := h.run(t, true)
	require.NoError(t, err)
	require.Zero(t, h.client.addNodeCalls)
	require.Zero(t, h.client.removeNodeCalls)
	require.Zero(t, h.client.closePortCalls)
	require.Zero(t, h.registrations.saveCalls)
	require.Zero(t, h.registrations.deleteCalls)
}

func TestRunDisableSSH_BackendNodeFailureStopsBeforeConfirmation(t *testing.T) {
	h := newDisableSSHTestHarness()
	h.client.getErr = errors.New("backend unavailable")

	_, _, err := h.run(t, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "disable SSH failed")
	require.Zero(t, h.confirmer.calls)
	require.Empty(t, h.client.revokeRequests)
}

func cloneRevokeRequest(req *nodev1.RevokeNodeSSHAccessRequest) *nodev1.RevokeNodeSSHAccessRequest {
	return &nodev1.RevokeNodeSSHAccessRequest{
		ExternalNodeId: req.GetExternalNodeId(),
		PortId:         req.GetPortId(),
		UserId:         req.GetUserId(),
		LinuxUser:      req.GetLinuxUser(),
	}
}

func testSSHAccess(userID, linuxUser, portID string) *nodev1.SSHAccess {
	return &nodev1.SSHAccess{UserId: userID, LinuxUser: linuxUser, PortId: portID}
}

func captureDisableSSHStdout(t *testing.T, run func(*terminal.Terminal) error) (string, error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = writer
	term := terminal.New()
	os.Stdout = oldStdout

	runErr := run(term)
	require.NoError(t, writer.Close())
	output, readErr := io.ReadAll(reader)
	require.NoError(t, readErr)
	require.NoError(t, reader.Close())
	return string(output), runErr
}
