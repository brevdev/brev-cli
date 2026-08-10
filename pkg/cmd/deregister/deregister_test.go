package deregister

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/externalnode"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type leaveTestPlatform struct {
	compatible bool
	events     *[]string
}

func (p *leaveTestPlatform) IsCompatible() bool {
	recordLeaveEvent(p.events, "platform")
	return p.compatible
}

type leaveTestStore struct {
	events           *[]string
	currentUserCalls int
	currentUserErr   error
}

func (s *leaveTestStore) GetCurrentUser() (*entity.User, error) {
	s.currentUserCalls++
	recordLeaveEvent(s.events, "auth")
	if s.currentUserErr != nil {
		return nil, s.currentUserErr
	}
	return &entity.User{ID: "user_current"}, nil
}

func (*leaveTestStore) GetAccessToken() (string, error) { return "token", nil }

type leaveTestRegistrationStore struct {
	events      *[]string
	reg         *register.DeviceRegistration
	loadErr     error
	deleteErr   error
	saveCalls   int
	deleteCalls int
}

func (s *leaveTestRegistrationStore) Save(*register.DeviceRegistration) error {
	s.saveCalls++
	recordLeaveEvent(s.events, "registration-save")
	return nil
}

func (s *leaveTestRegistrationStore) Load() (*register.DeviceRegistration, error) {
	recordLeaveEvent(s.events, "registration-load")
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.reg, nil
}

func (s *leaveTestRegistrationStore) Delete() error {
	s.deleteCalls++
	recordLeaveEvent(s.events, "registration-delete")
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.reg = nil
	return nil
}

func (s *leaveTestRegistrationStore) Exists() (bool, error) { return s.reg != nil, nil }

type leaveTestConfirmer struct {
	events *[]string
	answer bool
	calls  int
}

func (c *leaveTestConfirmer) ConfirmYesNo(string) bool {
	c.calls++
	recordLeaveEvent(c.events, "confirm")
	return c.answer
}

type leaveTestGater struct {
	events  *[]string
	calls   int
	reasons []string
	err     error
}

func (g *leaveTestGater) Gate(_ *terminal.Terminal, _ terminal.Confirmer, reason string, _ bool) error {
	g.calls++
	g.reasons = append(g.reasons, reason)
	recordLeaveEvent(g.events, "sudo")
	return g.err
}

type leaveTestNetBird struct {
	events *[]string
	calls  int
	err    error
}

func (n *leaveTestNetBird) Uninstall() error {
	n.calls++
	recordLeaveEvent(n.events, "netbird-uninstall")
	return n.err
}

type leaveRecordingClient struct {
	nodev1connect.ExternalNodeServiceClient

	events         *[]string
	listResponse   *nodev1.ListNodesResponse
	listErr        error
	returnNilList  bool
	removeErr      error
	listRequests   []*nodev1.ListNodesRequest
	removeRequests []*nodev1.RemoveNodeRequest
	revokeCalls    int
}

func (c *leaveRecordingClient) ListNodes(_ context.Context, req *connect.Request[nodev1.ListNodesRequest]) (*connect.Response[nodev1.ListNodesResponse], error) {
	recordLeaveEvent(c.events, "list-nodes")
	c.listRequests = append(c.listRequests, &nodev1.ListNodesRequest{OrganizationId: req.Msg.GetOrganizationId()})
	if c.listErr != nil {
		return nil, c.listErr
	}
	if c.returnNilList {
		return nil, nil
	}
	return connect.NewResponse(c.listResponse), nil
}

func (c *leaveRecordingClient) RemoveNode(_ context.Context, req *connect.Request[nodev1.RemoveNodeRequest]) (*connect.Response[nodev1.RemoveNodeResponse], error) {
	recordLeaveEvent(c.events, "remove-node")
	c.removeRequests = append(c.removeRequests, &nodev1.RemoveNodeRequest{ExternalNodeId: req.Msg.GetExternalNodeId()})
	if c.removeErr != nil {
		return nil, c.removeErr
	}
	return connect.NewResponse(&nodev1.RemoveNodeResponse{}), nil
}

func (c *leaveRecordingClient) RevokeNodeSSHAccess(context.Context, *connect.Request[nodev1.RevokeNodeSSHAccessRequest]) (*connect.Response[nodev1.RevokeNodeSSHAccessResponse], error) {
	c.revokeCalls++
	recordLeaveEvent(c.events, "revoke-ssh")
	return connect.NewResponse(&nodev1.RevokeNodeSSHAccessResponse{}), nil
}

type leaveTestNodeClientFactory struct {
	client nodev1connect.ExternalNodeServiceClient
}

func (f leaveTestNodeClientFactory) NewNodeClient(externalnode.TokenProvider, string) nodev1connect.ExternalNodeServiceClient {
	return f.client
}

type leaveTestHarness struct {
	events        []string
	store         *leaveTestStore
	registrations *leaveTestRegistrationStore
	confirmer     *leaveTestConfirmer
	gater         *leaveTestGater
	netbird       *leaveTestNetBird
	client        *leaveRecordingClient
	deps          leaveDeps
}

func newLeaveTestHarness() *leaveTestHarness {
	h := &leaveTestHarness{}
	reg := &register.DeviceRegistration{
		ExternalNodeID: "node_123",
		DisplayName:    "owned-node",
		OrgID:          "org_123",
		OrgName:        "owned-org",
	}
	node := &nodev1.ExternalNode{ExternalNodeId: reg.ExternalNodeID, Name: reg.DisplayName}
	h.store = &leaveTestStore{events: &h.events}
	h.registrations = &leaveTestRegistrationStore{events: &h.events, reg: reg}
	h.confirmer = &leaveTestConfirmer{events: &h.events, answer: true}
	h.gater = &leaveTestGater{events: &h.events}
	h.netbird = &leaveTestNetBird{events: &h.events}
	h.client = &leaveRecordingClient{
		events:       &h.events,
		listResponse: &nodev1.ListNodesResponse{Items: []*nodev1.ExternalNode{node}},
	}
	h.deps = leaveDeps{
		platform:          &leaveTestPlatform{compatible: true, events: &h.events},
		confirmer:         h.confirmer,
		gater:             h.gater,
		netbird:           h.netbird,
		nodeClients:       leaveTestNodeClientFactory{client: h.client},
		registrationStore: h.registrations,
	}
	return h
}

func (h *leaveTestHarness) run(t *testing.T, approve bool) (stdout string, stderr string, err error) {
	t.Helper()
	var warnings bytes.Buffer
	stdout, err = captureLeaveStdout(t, func(term *terminal.Terminal) error {
		return runLeave(context.Background(), term, &warnings, h.store, h.deps, approve)
	})
	return stdout, warnings.String(), err
}

func TestNewCmdLeave_CommandSurface(t *testing.T) {
	cmd := NewCmdLeave(terminal.New(), &leaveTestStore{})
	require.Equal(t, "leave", cmd.Use)
	require.Equal(t, []string{"deregister"}, cmd.Aliases)
	require.NotNil(t, cmd.Args)
	require.Contains(t, cmd.Annotations, "configuration")
	require.NotNil(t, cmd.Flags().Lookup("approve"))
}

func TestNewCmdDeregister_DeprecatedSourceCompatibility(t *testing.T) {
	var store DeregisterStore = &leaveTestStore{}
	cmd := NewCmdDeregister(terminal.New(), store)

	require.Equal(t, "leave", cmd.Name())
	require.Equal(t, []string{"deregister"}, cmd.Aliases)
}

func TestNewCmdLeave_DeregisterAliasWarnsOnExecution(t *testing.T) {
	h := newLeaveTestHarness()
	var stderr bytes.Buffer
	_, err := captureLeaveStdout(t, func(term *terminal.Terminal) error {
		root := &cobra.Command{Use: "brev"}
		root.AddCommand(newCmdLeave(term, h.store, h.deps))
		root.SetArgs([]string{"deregister", "--approve"})
		root.SetOut(io.Discard)
		root.SetErr(&stderr)
		return root.Execute()
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(stderr.String(), "Warning: \"brev deregister\" is deprecated; use \"brev leave\" instead.\n"+
		"This command no longer removes SSH keys; run \"brev disable-ssh\" before leaving if you want to remove Brev-managed SSH access.\n"))
}

func TestNewCmdLeave_HelpDoesNotWarn(t *testing.T) {
	for _, name := range []string{"leave", "deregister"} {
		t.Run(name, func(t *testing.T) {
			h := newLeaveTestHarness()
			var stderr bytes.Buffer
			root := &cobra.Command{Use: "brev"}
			root.AddCommand(newCmdLeave(terminal.New(), h.store, h.deps))
			root.SetArgs([]string{name, "--help"})
			root.SetOut(io.Discard)
			root.SetErr(&stderr)

			require.NoError(t, root.Execute())
			require.NotContains(t, stderr.String(), "deprecated")
			require.Empty(t, h.events)
		})
	}
}

func TestNewCmdLeave_CanonicalInvocationDoesNotWarnAboutDeprecation(t *testing.T) {
	h := newLeaveTestHarness()
	var stderr bytes.Buffer
	_, err := captureLeaveStdout(t, func(term *terminal.Terminal) error {
		root := &cobra.Command{Use: "brev"}
		root.AddCommand(newCmdLeave(term, h.store, h.deps))
		root.SetArgs([]string{"leave", "--approve"})
		root.SetOut(io.Discard)
		root.SetErr(&stderr)
		return root.Execute()
	})
	require.NoError(t, err)
	require.NotContains(t, stderr.String(), "deprecated")
	require.Contains(t, stderr.String(), "may interrupt commands using Brev SSH")
}

func TestNewCmdLeave_RejectsArguments(t *testing.T) {
	for _, name := range []string{"leave", "deregister"} {
		t.Run(name, func(t *testing.T) {
			h := newLeaveTestHarness()
			root := &cobra.Command{Use: "brev"}
			root.AddCommand(newCmdLeave(terminal.New(), h.store, h.deps))
			root.SetArgs([]string{name, "unexpected"})
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			require.Error(t, root.Execute())
			require.Empty(t, h.events)
		})
	}
}

func TestRunLeave_RemainingGrantsWarnButDoNotBlock(t *testing.T) {
	h := newLeaveTestHarness()
	h.client.listResponse.Items[0].SshAccess = []*nodev1.SSHAccess{
		{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_1"},
		nil,
		{UserId: "user_2", LinuxUser: "ubuntu", PortId: "port_2"},
		{UserId: "user_3", LinuxUser: "alice", PortId: "port_3"},
	}

	_, stderr, err := h.run(t, false)
	require.NoError(t, err)
	require.Contains(t, stderr, "3 SSH grants across 2 Linux accounts")
	require.Contains(t, stderr, `Leaving stops Brev-routed SSH but does not remove keys from authorized_keys. Cancel and run "brev disable-ssh" first if you want Brev-managed SSH credentials removed.`)
	require.Len(t, h.client.removeRequests, 1)
}

func TestRunLeave_ApproveSkipsConfirmationButNotWarnings(t *testing.T) {
	h := newLeaveTestHarness()
	h.client.listResponse.Items[0].SshAccess = []*nodev1.SSHAccess{
		{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_1"},
		{UserId: "user_2", LinuxUser: "alice", PortId: "port_2"},
	}
	_, stderr, err := h.run(t, true)
	require.NoError(t, err)
	require.Zero(t, h.confirmer.calls)
	require.Contains(t, stderr, "may interrupt commands using Brev SSH")
	require.Contains(t, stderr, "2 SSH grants across 2 Linux accounts")
	require.Contains(t, stderr, `run "brev disable-ssh" first`)
	require.Equal(t, 1, h.gater.calls)
}

func TestRunLeave_CancelStopsBeforeSudoAndMutation(t *testing.T) {
	h := newLeaveTestHarness()
	h.confirmer.answer = false

	stdout, _, err := h.run(t, false)
	require.NoError(t, err)
	require.Equal(t, []string{"platform", "registration-load", "auth", "list-nodes", "confirm"}, h.events)
	require.Zero(t, h.gater.calls)
	require.Empty(t, h.client.removeRequests)
	require.Zero(t, h.netbird.calls)
	require.Zero(t, h.registrations.deleteCalls)
	require.NotContains(t, stdout, "Left the Brev network.")
}

func TestRunLeave_IncompatiblePlatformStopsBeforeLoadOrMutation(t *testing.T) {
	h := newLeaveTestHarness()
	h.deps.platform = &leaveTestPlatform{compatible: false, events: &h.events}

	stdout, _, err := h.run(t, false)
	require.EqualError(t, err, "brev leave is only supported on Linux")
	require.Equal(t, []string{"platform"}, h.events)
	require.NotNil(t, h.registrations.reg)
	require.Zero(t, h.confirmer.calls)
	require.Zero(t, h.gater.calls)
	require.Empty(t, h.client.removeRequests)
	require.Zero(t, h.netbird.calls)
	require.Zero(t, h.registrations.deleteCalls)
	require.NotContains(t, stdout, "Left the Brev network.")
}

func TestRunLeave_AuthenticationFailureStopsBeforeLookupOrMutation(t *testing.T) {
	h := newLeaveTestHarness()
	authErr := errors.New("authentication failed")
	h.store.currentUserErr = authErr

	stdout, _, err := h.run(t, false)
	require.ErrorIs(t, err, authErr)
	require.Equal(t, []string{"platform", "registration-load", "auth"}, h.events)
	require.NotNil(t, h.registrations.reg)
	require.Empty(t, h.client.listRequests)
	require.Zero(t, h.confirmer.calls)
	require.Zero(t, h.gater.calls)
	require.Empty(t, h.client.removeRequests)
	require.Zero(t, h.netbird.calls)
	require.Zero(t, h.registrations.deleteCalls)
	require.NotContains(t, stdout, "Left the Brev network.")
}

func TestRunLeave_SudoFailureStopsBeforeAuthoritativeMutation(t *testing.T) {
	h := newLeaveTestHarness()
	sudoErr := errors.New("sudo unavailable")
	h.gater.err = sudoErr

	stdout, _, err := h.run(t, true)
	require.ErrorIs(t, err, sudoErr)
	require.Equal(t, []string{"platform", "registration-load", "auth", "list-nodes", "sudo"}, h.events)
	require.NotNil(t, h.registrations.reg)
	require.Empty(t, h.client.removeRequests)
	require.Zero(t, h.netbird.calls)
	require.Zero(t, h.registrations.deleteCalls)
	require.NotContains(t, stdout, "Left the Brev network.")
}

func TestRunLeave_OrderIsRemoveNodeUninstallDeleteRegistration(t *testing.T) {
	h := newLeaveTestHarness()

	stdout, _, err := h.run(t, false)
	require.NoError(t, err)
	require.Equal(t, []string{
		"platform", "registration-load", "auth", "list-nodes", "confirm", "sudo",
		"remove-node", "netbird-uninstall", "registration-delete",
	}, h.events)
	require.Equal(t, []string{"Leave Brev network"}, h.gater.reasons)
	require.Equal(t, []*nodev1.ListNodesRequest{{OrganizationId: "org_123"}}, h.client.listRequests)
	require.Equal(t, []*nodev1.RemoveNodeRequest{{ExternalNodeId: "node_123"}}, h.client.removeRequests)
	require.Contains(t, stdout, "Left the Brev network.")
}

func TestRunLeave_CompleteNodeListWithoutRegisteredIDAllowsAuthoritativeRemoveRetry(t *testing.T) {
	h := newLeaveTestHarness()
	h.client.listResponse = &nodev1.ListNodesResponse{Items: []*nodev1.ExternalNode{nil, {ExternalNodeId: "other"}}}

	_, stderr, err := h.run(t, true)
	require.NoError(t, err)
	require.Contains(t, stderr, "backend node is already absent")
	require.Contains(t, stderr, "tagged host keys may remain")
	require.Len(t, h.client.removeRequests, 1)
}

func TestRunLeave_ListPermissionDeniedStopsBeforeConfirmationAndMutation(t *testing.T) {
	h := newLeaveTestHarness()
	h.client.listErr = connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
	assertLeaveLookupFailureStopsMutation(t, h)
}

func TestRunLeave_RegisteredIDAbsentFromIncompleteListStopsBeforeMutation(t *testing.T) {
	h := newLeaveTestHarness()
	h.client.listResponse = &nodev1.ListNodesResponse{
		Items:         []*nodev1.ExternalNode{{ExternalNodeId: "other"}},
		NextPageToken: "next",
	}
	assertLeaveLookupFailureStopsMutation(t, h)
}

func TestRunLeave_OtherLookupFailureStopsBeforeConfirmationAndMutation(t *testing.T) {
	h := newLeaveTestHarness()
	h.client.listErr = errors.New("backend unavailable")
	assertLeaveLookupFailureStopsMutation(t, h)
}

func TestRunLeave_EmptyLookupResponseStopsBeforeConfirmationAndMutation(t *testing.T) {
	h := newLeaveTestHarness()
	h.client.returnNilList = true
	assertLeaveLookupFailureStopsMutation(t, h)
}

func TestRunLeave_EmptyLookupMessageStopsBeforeConfirmationAndMutation(t *testing.T) {
	h := newLeaveTestHarness()
	h.client.listResponse = nil
	assertLeaveLookupFailureStopsMutation(t, h)
}

func TestRunLeave_RemoveNodeNotFoundIsAccepted(t *testing.T) {
	h := newLeaveTestHarness()
	h.client.removeErr = connect.NewError(connect.CodeNotFound, errors.New("already absent"))

	stdout, _, err := h.run(t, true)
	require.NoError(t, err)
	require.Equal(t, 1, h.netbird.calls)
	require.Equal(t, 1, h.registrations.deleteCalls)
	require.Contains(t, stdout, "Left the Brev network.")
}

func TestRunLeave_RemoveNodeFailureStopsLocalTeardown(t *testing.T) {
	h := newLeaveTestHarness()
	removeErr := errors.New("remove failed")
	h.client.removeErr = connect.NewError(connect.CodeInternal, removeErr)

	stdout, _, err := h.run(t, true)
	require.ErrorIs(t, err, removeErr)
	require.Zero(t, h.netbird.calls)
	require.Zero(t, h.registrations.deleteCalls)
	require.NotContains(t, stdout, "Left the Brev network.")
}

func TestRunLeave_NetBirdFailureReturnsErrorAndRetainsRegistration(t *testing.T) {
	h := newLeaveTestHarness()
	netbirdErr := errors.New("uninstall failed")
	h.netbird.err = netbirdErr

	stdout, _, err := h.run(t, true)
	require.ErrorIs(t, err, netbirdErr)
	require.Zero(t, h.registrations.deleteCalls)
	require.NotNil(t, h.registrations.reg)
	require.NotContains(t, stdout, "Left the Brev network.")
}

func TestRunLeave_RegistrationDeleteFailureReturnsErrorAndNoSuccess(t *testing.T) {
	h := newLeaveTestHarness()
	deleteErr := errors.New("delete failed")
	h.registrations.deleteErr = deleteErr

	stdout, _, err := h.run(t, true)
	require.ErrorIs(t, err, deleteErr)
	require.Equal(t, 1, h.registrations.deleteCalls)
	require.NotNil(t, h.registrations.reg)
	require.NotContains(t, stdout, "Left the Brev network.")
}

func TestRunLeave_NeverRevokesSSHOrSavesRegistration(t *testing.T) {
	h := newLeaveTestHarness()
	h.client.listResponse.Items[0].SshAccess = []*nodev1.SSHAccess{{UserId: "user_1", LinuxUser: "ubuntu", PortId: "port_1"}}

	_, _, err := h.run(t, true)
	require.NoError(t, err)
	require.Zero(t, h.client.revokeCalls)
	require.Zero(t, h.registrations.saveCalls)
	require.NotContains(t, h.events, "revoke-ssh")
}

func TestRunLeave_RegistrationLoadFailureDoesNotAuthenticate(t *testing.T) {
	h := newLeaveTestHarness()
	h.registrations.loadErr = errors.New("registration missing")

	stdout, _, err := h.run(t, false)
	require.Error(t, err)
	require.Equal(t, []string{"platform", "registration-load"}, h.events)
	require.Zero(t, h.store.currentUserCalls)
	require.NotNil(t, h.registrations.reg)
	require.Empty(t, h.client.removeRequests)
	require.Zero(t, h.netbird.calls)
	require.Zero(t, h.registrations.deleteCalls)
	require.NotContains(t, stdout, "Left the Brev network.")
}

func assertLeaveLookupFailureStopsMutation(t *testing.T, h *leaveTestHarness) {
	t.Helper()
	stdout, _, err := h.run(t, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "inspect joined node before leaving")
	require.Equal(t, []string{"platform", "registration-load", "auth", "list-nodes"}, h.events)
	require.Zero(t, h.confirmer.calls)
	require.Zero(t, h.gater.calls)
	require.Empty(t, h.client.removeRequests)
	require.Zero(t, h.netbird.calls)
	require.Zero(t, h.registrations.deleteCalls)
	require.NotNil(t, h.registrations.reg)
	require.NotContains(t, stdout, "Left the Brev network.")
}

func recordLeaveEvent(events *[]string, event string) {
	if events != nil {
		*events = append(*events, event)
	}
}

func captureLeaveStdout(t *testing.T, run func(*terminal.Terminal) error) (string, error) {
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
