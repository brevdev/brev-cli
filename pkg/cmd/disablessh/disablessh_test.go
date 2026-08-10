package disablessh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type disableSSHTestPlatform struct {
	compatible bool
	events     *[]string
}

func (p *disableSSHTestPlatform) IsCompatible() bool {
	recordDisableSSHEvent(p.events, "platform")
	return p.compatible
}

type disableSSHTestStore struct {
	events           *[]string
	currentUserCalls int
	accessTokenCalls int
	currentUserErr   error
}

func (s *disableSSHTestStore) GetCurrentUser() (*entity.User, error) {
	s.currentUserCalls++
	recordDisableSSHEvent(s.events, "auth")
	if s.currentUserErr != nil {
		return nil, s.currentUserErr
	}
	return &entity.User{ID: "user_current"}, nil
}

func (s *disableSSHTestStore) GetAccessToken() (string, error) {
	s.accessTokenCalls++
	return "token", nil
}

type disableSSHTestRegistrationStore struct {
	events      *[]string
	exists      bool
	existsErr   error
	loadErr     error
	reg         *register.DeviceRegistration
	saveCalls   int
	deleteCalls int
}

func (s *disableSSHTestRegistrationStore) Exists() (bool, error) {
	recordDisableSSHEvent(s.events, "registration-exists")
	return s.exists, s.existsErr
}

func (s *disableSSHTestRegistrationStore) Load() (*register.DeviceRegistration, error) {
	recordDisableSSHEvent(s.events, "registration-load")
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.reg, nil
}

func (s *disableSSHTestRegistrationStore) Save(*register.DeviceRegistration) error {
	s.saveCalls++
	recordDisableSSHEvent(s.events, "registration-save")
	return nil
}

func (s *disableSSHTestRegistrationStore) Delete() error {
	s.deleteCalls++
	recordDisableSSHEvent(s.events, "registration-delete")
	return nil
}

type disableSSHTestConfirmer struct {
	events *[]string
	answer bool
	calls  int
	labels []string
}

func (c *disableSSHTestConfirmer) ConfirmYesNo(label string) bool {
	c.calls++
	c.labels = append(c.labels, label)
	recordDisableSSHEvent(c.events, "confirm")
	return c.answer
}

type disableSSHTestGater struct {
	events  *[]string
	calls   int
	reasons []string
	err     error
}

func (g *disableSSHTestGater) Gate(_ *terminal.Terminal, _ terminal.Confirmer, reason string, _ bool) error {
	g.calls++
	g.reasons = append(g.reasons, reason)
	recordDisableSSHEvent(g.events, "sudo")
	return g.err
}

type disableSSHTestTunnel struct {
	events         *[]string
	ensureCalls    int
	uninstallCalls int
	err            error
}

func (t *disableSSHTestTunnel) EnsureConnected(context.Context) error {
	t.ensureCalls++
	recordDisableSSHEvent(t.events, "tunnel")
	return t.err
}

// Uninstall is deliberately outside register.NetBirdConnector. It makes an
// accidental concrete-type assertion or broadened dependency observable.
func (t *disableSSHTestTunnel) Uninstall() error {
	t.uninstallCalls++
	recordDisableSSHEvent(t.events, "netbird-uninstall")
	return nil
}

type disableSSHTestKeyCleaner struct {
	events *[]string
	result KeyCleanupResult
	err    error
	calls  int
}

func (c *disableSSHTestKeyCleaner) RemoveBrevKeys(context.Context) (KeyCleanupResult, error) {
	c.calls++
	recordDisableSSHEvent(c.events, "cleanup")
	return c.result, c.err
}

type disableSSHRecordingClient struct {
	nodev1connect.ExternalNodeServiceClient

	events *[]string
	node   *nodev1.ExternalNode
	getErr error

	mu               sync.Mutex
	revokeRequests   []*nodev1.RevokeNodeSSHAccessRequest
	revokeErrors     map[int]error
	activeRevokes    int
	maxActiveRevokes int

	addNodeCalls    int
	removeNodeCalls int
	closePortCalls  int
}

func (c *disableSSHRecordingClient) GetNode(_ context.Context, req *connect.Request[nodev1.GetNodeRequest]) (*connect.Response[nodev1.GetNodeResponse], error) {
	recordDisableSSHEvent(c.events, "get-node")
	if c.getErr != nil {
		return nil, c.getErr
	}
	if req.Msg.GetExternalNodeId() != "node_123" || req.Msg.GetOrganizationId() != "org_123" {
		return nil, fmt.Errorf("unexpected GetNode request: %+v", req.Msg)
	}
	return connect.NewResponse(&nodev1.GetNodeResponse{ExternalNode: c.node}), nil
}

func (c *disableSSHRecordingClient) RevokeNodeSSHAccess(_ context.Context, req *connect.Request[nodev1.RevokeNodeSSHAccessRequest]) (*connect.Response[nodev1.RevokeNodeSSHAccessResponse], error) {
	c.mu.Lock()
	callIndex := len(c.revokeRequests)
	c.revokeRequests = append(c.revokeRequests, cloneRevokeRequest(req.Msg))
	c.activeRevokes++
	if c.activeRevokes > c.maxActiveRevokes {
		c.maxActiveRevokes = c.activeRevokes
	}
	c.mu.Unlock()

	recordDisableSSHEvent(c.events, "revoke:"+req.Msg.GetUserId())
	time.Sleep(time.Millisecond)

	c.mu.Lock()
	c.activeRevokes--
	err := c.revokeErrors[callIndex]
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&nodev1.RevokeNodeSSHAccessResponse{}), nil
}

func (c *disableSSHRecordingClient) AddNode(context.Context, *connect.Request[nodev1.AddNodeRequest]) (*connect.Response[nodev1.AddNodeResponse], error) {
	c.addNodeCalls++
	recordDisableSSHEvent(c.events, "add-node")
	return connect.NewResponse(&nodev1.AddNodeResponse{}), nil
}

func (c *disableSSHRecordingClient) RemoveNode(context.Context, *connect.Request[nodev1.RemoveNodeRequest]) (*connect.Response[nodev1.RemoveNodeResponse], error) {
	c.removeNodeCalls++
	recordDisableSSHEvent(c.events, "remove-node")
	return connect.NewResponse(&nodev1.RemoveNodeResponse{}), nil
}

func (c *disableSSHRecordingClient) ClosePort(context.Context, *connect.Request[nodev1.ClosePortRequest]) (*connect.Response[nodev1.ClosePortResponse], error) {
	c.closePortCalls++
	recordDisableSSHEvent(c.events, "close-port")
	return connect.NewResponse(&nodev1.ClosePortResponse{}), nil
}

type disableSSHTestNodeClientFactory struct {
	client nodev1connect.ExternalNodeServiceClient
}

func (f disableSSHTestNodeClientFactory) NewNodeClient(externalnode.TokenProvider, string) nodev1connect.ExternalNodeServiceClient {
	return f.client
}

type disableSSHTestHarness struct {
	events        []string
	store         *disableSSHTestStore
	registrations *disableSSHTestRegistrationStore
	confirmer     *disableSSHTestConfirmer
	gater         *disableSSHTestGater
	tunnel        *disableSSHTestTunnel
	cleaner       *disableSSHTestKeyCleaner
	client        *disableSSHRecordingClient
	deps          disableSSHDeps
}

func newDisableSSHTestHarness(accesses ...*nodev1.SSHAccess) *disableSSHTestHarness {
	h := &disableSSHTestHarness{}
	h.store = &disableSSHTestStore{events: &h.events}
	h.registrations = &disableSSHTestRegistrationStore{
		events: &h.events,
		exists: true,
		reg: &register.DeviceRegistration{
			ExternalNodeID: "node_123",
			DisplayName:    "owned-node",
			OrgID:          "org_123",
			OrgName:        "owned-org",
		},
	}
	h.confirmer = &disableSSHTestConfirmer{events: &h.events, answer: true}
	h.gater = &disableSSHTestGater{events: &h.events}
	h.tunnel = &disableSSHTestTunnel{events: &h.events}
	h.cleaner = &disableSSHTestKeyCleaner{
		events: &h.events,
		result: KeyCleanupResult{AccountsScanned: 4, AccountsChanged: 2, KeysRemoved: 3},
	}
	h.client = &disableSSHRecordingClient{
		events:       &h.events,
		node:         &nodev1.ExternalNode{ExternalNodeId: "node_123", Name: "owned-node", SshAccess: accesses},
		revokeErrors: make(map[int]error),
	}
	h.deps = disableSSHDeps{
		platform:          &disableSSHTestPlatform{compatible: true, events: &h.events},
		confirmer:         h.confirmer,
		gater:             h.gater,
		tunnel:            h.tunnel,
		nodeClients:       disableSSHTestNodeClientFactory{client: h.client},
		registrationStore: h.registrations,
		keyCleaner:        h.cleaner,
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
	require.Equal(t, "Disable all Brev-managed SSH access on this node", cmd.Short)
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
	require.Contains(t, err.Error(), "unknown command")
	require.Empty(t, h.events)
}

func TestRunDisableSSH_MissingRegistrationDoesNotAuthenticateOrCallRPC(t *testing.T) {
	h := newDisableSSHTestHarness()
	h.registrations.exists = false

	_, _, err := h.run(t, false)
	require.EqualError(t, err, `This machine has not joined a Brev network; run "brev join" first.`)
	require.Equal(t, []string{"platform", "registration-exists"}, h.events)
	require.Zero(t, h.store.currentUserCalls)
	require.Empty(t, h.client.revokeRequests)
	require.Zero(t, h.tunnel.ensureCalls)
	require.Zero(t, h.gater.calls)
	require.Zero(t, h.cleaner.calls)
}

func TestRunDisableSSH_CancelStopsBeforeSudoTunnelRevocationAndCleanup(t *testing.T) {
	h := newDisableSSHTestHarness(testSSHAccess("user_1", "ubuntu", "port_1"))
	h.confirmer.answer = false

	_, _, err := h.run(t, false)
	require.NoError(t, err)
	require.Equal(t, []string{"platform", "registration-exists", "registration-load", "auth", "get-node", "confirm"}, h.events)
	require.Zero(t, h.gater.calls)
	require.Zero(t, h.tunnel.ensureCalls)
	require.Empty(t, h.client.revokeRequests)
	require.Zero(t, h.cleaner.calls)
}

func TestRunDisableSSH_ApproveSkipsConfirmationButPrintsSafetyWarning(t *testing.T) {
	h := newDisableSSHTestHarness()

	_, stderr, err := h.run(t, true)
	require.NoError(t, err)
	require.Zero(t, h.confirmer.calls)
	require.Contains(t, stderr, "node-wide")
	require.Contains(t, stderr, "active SSH sessions are not forcibly terminated")
	require.Equal(t, 1, h.cleaner.calls)
}

func TestRunDisableSSH_ShowsGrantAndDistinctLinuxAccountCounts(t *testing.T) {
	h := newDisableSSHTestHarness(
		testSSHAccess("user_1", "ubuntu", "port_1"),
		testSSHAccess("user_2", "ubuntu", "port_2"),
		testSSHAccess("user_3", "alice", "port_3"),
	)

	stdout, _, err := h.run(t, true)
	require.NoError(t, err)
	for _, text := range []string{"owned-node", "node_123", "SSH grants:    3", "Linux accounts: 2"} {
		require.Contains(t, stdout, text)
	}
}

func TestRunDisableSSH_IgnoresNilAccessEntries(t *testing.T) {
	h := newDisableSSHTestHarness(
		testSSHAccess("user_1", "ubuntu", "port_1"),
		nil,
		testSSHAccess("user_2", "alice", "port_2"),
	)

	stdout, _, err := h.run(t, true)
	require.NoError(t, err)
	require.Contains(t, stdout, "SSH grants:    2")
	require.Len(t, h.client.revokeRequests, 2)
}

func TestRunDisableSSH_ConnectsBeforeFirstRevocation(t *testing.T) {
	h := newDisableSSHTestHarness(testSSHAccess("user_1", "ubuntu", "port_1"))

	_, _, err := h.run(t, true)
	require.NoError(t, err)
	requireOrderedSubsequence(t, h.events, "sudo", "tunnel", "revoke:user_1", "cleanup")
}

func TestRunDisableSSH_RevokesEveryExactTupleSequentiallyOnce(t *testing.T) {
	accesses := []*nodev1.SSHAccess{
		testSSHAccess("user_1", "ubuntu", "port_1"),
		testSSHAccess("user_2", "ubuntu", "port_2"),
		testSSHAccess("user_3", "alice", "port_3"),
	}
	h := newDisableSSHTestHarness(accesses...)

	_, _, err := h.run(t, true)
	require.NoError(t, err)
	require.Equal(t, 1, h.client.maxActiveRevokes)
	require.Len(t, h.client.revokeRequests, len(accesses))
	for i, access := range accesses {
		require.Equal(t, &nodev1.RevokeNodeSSHAccessRequest{
			ExternalNodeId: "node_123",
			PortId:         access.GetPortId(),
			UserId:         access.GetUserId(),
			LinuxUser:      access.GetLinuxUser(),
		}, h.client.revokeRequests[i])
	}
}

func TestRunDisableSSH_ContinuesAfterMiddleRevocationFailureAndJoinsErrors(t *testing.T) {
	firstErr := errors.New("first revoke failed")
	middleErr := errors.New("middle revoke failed")
	h := newDisableSSHTestHarness(
		testSSHAccess("user_1", "ubuntu", "port_1"),
		testSSHAccess("user_2", "alice", "port_2"),
		testSSHAccess("user_3", "carol", "port_3"),
	)
	h.client.revokeErrors[0] = firstErr
	h.client.revokeErrors[1] = middleErr

	_, _, err := h.run(t, true)
	require.Error(t, err)
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, middleErr)
	require.Contains(t, err.Error(), "disable SSH backend cleanup incomplete")
	for _, text := range []string{"user_1", "ubuntu", "port_1", "user_2", "alice", "port_2"} {
		require.Contains(t, err.Error(), text)
	}
	require.Len(t, h.client.revokeRequests, 3)
	require.Equal(t, []string{"revoke:user_1", "revoke:user_2", "revoke:user_3"}, filterDisableSSHEvents(h.events, "revoke:"))
}

func TestRunDisableSSH_AnyRevocationFailureBlocksLocalCleanup(t *testing.T) {
	h := newDisableSSHTestHarness(
		testSSHAccess("user_1", "ubuntu", "port_1"),
		testSSHAccess("user_2", "alice", "port_2"),
	)
	h.client.revokeErrors[0] = errors.New("revocation failed")

	_, _, err := h.run(t, true)
	require.Error(t, err)
	require.Len(t, h.client.revokeRequests, 2)
	require.Zero(t, h.cleaner.calls)
}

func TestRunDisableSSH_NotFoundRevocationBlocksLocalCleanup(t *testing.T) {
	h := newDisableSSHTestHarness(testSSHAccess("user_1", "ubuntu", "port_1"))
	h.client.revokeErrors[0] = connect.NewError(connect.CodeNotFound, errors.New("port missing"))

	_, _, err := h.run(t, true)
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.Zero(t, h.cleaner.calls)
}

func TestRunDisableSSH_NoGrantsSkipsTunnelAndStillCleansOrphanedKeys(t *testing.T) {
	h := newDisableSSHTestHarness()

	_, _, err := h.run(t, true)
	require.NoError(t, err)
	require.Zero(t, h.tunnel.ensureCalls)
	require.Empty(t, h.client.revokeRequests)
	require.Equal(t, 1, h.cleaner.calls)
	requireOrderedSubsequence(t, h.events, "sudo", "cleanup")
}

func TestRunDisableSSH_TunnelFailureStopsBeforeRevocationAndCleanup(t *testing.T) {
	h := newDisableSSHTestHarness(testSSHAccess("user_1", "ubuntu", "port_1"))
	tunnelErr := errors.New("tunnel unavailable")
	h.tunnel.err = tunnelErr

	_, _, err := h.run(t, true)
	require.ErrorIs(t, err, tunnelErr)
	require.Contains(t, err.Error(), "connected Brev tunnel")
	require.Empty(t, h.client.revokeRequests)
	require.Zero(t, h.cleaner.calls)
}

func TestRunDisableSSH_LocalCleanupFailureReturnsErrorAndPreservesMembership(t *testing.T) {
	h := newDisableSSHTestHarness(testSSHAccess("user_1", "ubuntu", "port_1"))
	cleanupErr := errors.New("local cleanup failed")
	h.cleaner.err = cleanupErr

	_, _, err := h.run(t, true)
	require.ErrorIs(t, err, cleanupErr)
	require.Contains(t, err.Error(), "disable SSH local key cleanup incomplete")
	require.Equal(t, 1, h.cleaner.calls)
	require.Zero(t, h.client.removeNodeCalls)
	require.Zero(t, h.registrations.deleteCalls)
	require.Zero(t, h.tunnel.uninstallCalls)
}

func TestRunDisableSSH_DoesNotRemoveNodeClosePortUninstallNetBirdOrDeleteRegistration(t *testing.T) {
	h := newDisableSSHTestHarness(testSSHAccess("user_1", "ubuntu", "port_1"))

	_, _, err := h.run(t, true)
	require.NoError(t, err)
	require.Zero(t, h.client.removeNodeCalls)
	require.Zero(t, h.client.closePortCalls)
	require.Zero(t, h.client.addNodeCalls)
	require.Zero(t, h.tunnel.uninstallCalls)
	require.Zero(t, h.registrations.deleteCalls)
	require.Zero(t, h.registrations.saveCalls)
}

func TestRunDisableSSH_SuccessIncludesCleanupCounts(t *testing.T) {
	h := newDisableSSHTestHarness()

	stdout, _, err := h.run(t, true)
	require.NoError(t, err)
	require.Contains(t, stdout, "3")
	require.Contains(t, stdout, "keys removed")
	require.Contains(t, stdout, "2")
	require.Contains(t, stdout, "accounts changed")
}

func TestRunDisableSSH_StateMachineOrdersPreflightConfirmationAndSudo(t *testing.T) {
	h := newDisableSSHTestHarness(testSSHAccess("user_1", "ubuntu", "port_1"))

	_, _, err := h.run(t, false)
	require.NoError(t, err)
	require.Equal(t, []string{
		"platform",
		"registration-exists",
		"registration-load",
		"auth",
		"get-node",
		"confirm",
		"sudo",
		"tunnel",
		"revoke:user_1",
		"cleanup",
	}, h.events)
	require.Equal(t, []string{"Node-wide Brev SSH cleanup"}, h.gater.reasons)
}

func TestRunDisableSSH_BackendNodeFailureStopsBeforeConfirmationAndMutation(t *testing.T) {
	h := newDisableSSHTestHarness()
	h.client.getErr = errors.New("backend unavailable")

	_, _, err := h.run(t, false)
	require.Error(t, err)
	require.Equal(t, []string{"platform", "registration-exists", "registration-load", "auth", "get-node"}, h.events)
	require.Zero(t, h.confirmer.calls)
	require.Zero(t, h.gater.calls)
	require.Zero(t, h.tunnel.ensureCalls)
	require.Zero(t, h.cleaner.calls)
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

func recordDisableSSHEvent(events *[]string, event string) {
	if events != nil {
		*events = append(*events, event)
	}
}

func filterDisableSSHEvents(events []string, prefix string) []string {
	var filtered []string
	for _, event := range events {
		if strings.HasPrefix(event, prefix) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func requireOrderedSubsequence(t *testing.T, events []string, expected ...string) {
	t.Helper()
	next := 0
	for _, event := range events {
		if next < len(expected) && event == expected[next] {
			next++
		}
	}
	require.Equal(t, len(expected), next, "events %v do not contain ordered subsequence %v", events, expected)
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
