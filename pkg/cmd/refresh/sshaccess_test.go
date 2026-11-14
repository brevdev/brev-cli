package refresh

import (
	"context"
	"errors"
	"testing"
	"time"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type stubEnvironmentSSHClient struct {
	environment    *devplanev1.Environment
	networkInfo    *devplanev1.EnvironmentNetworkInfo
	err            error
	request        *devplanev1.GetEnvironmentRequest
	networkRequest *devplanev1.EnvironmentServiceGetNetworkInfoRequest
}

func (s *stubEnvironmentSSHClient) GetEnvironment(
	_ context.Context,
	req *connect.Request[devplanev1.GetEnvironmentRequest],
) (*connect.Response[devplanev1.GetEnvironmentResponse], error) {
	s.request = req.Msg
	if s.err != nil {
		return nil, s.err
	}
	return connect.NewResponse(&devplanev1.GetEnvironmentResponse{Environment: s.environment}), nil
}

func (s *stubEnvironmentSSHClient) GetNetworkInfo(
	_ context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceGetNetworkInfoRequest],
) (*connect.Response[devplanev1.EnvironmentServiceGetNetworkInfoResponse], error) {
	s.networkRequest = req.Msg
	return connect.NewResponse(&devplanev1.EnvironmentServiceGetNetworkInfoResponse{
		NetworkInfo: s.networkInfo,
	}), nil
}

func TestEnrichWorkspacesWithSSHAccess_UsesCurrentUsersPort(t *testing.T) {
	workspace := entity.Workspace{
		ID:                   "env-1",
		Name:                 "container-env",
		DNS:                  "legacy.example.com",
		Status:               entity.Running,
		SSHUser:              "ubuntu",
		SSHPort:              22,
		HostSSHUser:          "ubuntu",
		HostSSHPort:          22,
		SSHProxyHostname:     "legacy-proxy.example.com",
		HostSSHProxyHostname: "legacy-host-proxy.example.com",
	}
	client := &stubEnvironmentSSHClient{
		environment: &devplanev1.Environment{
			Instance: &devplanev1.Instance{
				SshHostname: "203.0.113.10",
				SshPort:     22,
				PublicIp:    "203.0.113.10",
			},
			SshAccess: []*devplanev1.SSHAccess{
				{UserId: "other-user", LinuxUser: "wrong-user", PortId: "other-port"},
				{UserId: "user-1", LinuxUser: "root", PortId: "ssh-port"},
			},
		},
		networkInfo: &devplanev1.EnvironmentNetworkInfo{
			Ports: []*devplanev1.Port{
				{PortId: "other-port", Hostname: strPtr("wrong.example.com"), PortNumber: 49999},
				{PortId: "ssh-port", Hostname: strPtr("skybridge.example.com"), PortNumber: 41234, ServerPort: 22},
			},
		},
	}

	got := enrichWorkspacesWithSSHAccess(context.Background(), client, "user-1", []entity.Workspace{workspace})
	want := workspace
	want.SSHHostname = "skybridge.example.com"
	want.SSHPort = 41234
	want.SSHUser = "root"
	want.SSHProxyHostname = ""
	want.HostSSHHostname = "203.0.113.10"
	want.HostSSHProxyHostname = ""
	want.PortID = "ssh-port"
	want.SSHCertEligible = false // mock environment has no certauth label

	if diff := cmp.Diff([]entity.Workspace{want}, got); diff != "" {
		t.Fatalf("unexpected workspace (-want +got): %s", diff)
	}
	if client.request.GetEnvironmentId() != workspace.ID {
		t.Fatalf("requested environment %q, want %q", client.request.GetEnvironmentId(), workspace.ID)
	}
	options := client.request.GetAttachedDataOptions()
	if !options.GetInstance() || !options.GetSshAccess() || options.GetSysUsers() {
		t.Fatalf("missing SSH attachment options: %+v", options)
	}
	if client.networkRequest.GetEnvironmentId() != workspace.ID {
		t.Fatalf("requested network environment %q, want %q", client.networkRequest.GetEnvironmentId(), workspace.ID)
	}
}

func TestEnrichWorkspacesWithSSHAccess_FallsBackOnError(t *testing.T) {
	workspace := entity.Workspace{
		ID:      "env-1",
		Name:    "legacy-env",
		DNS:     "legacy.example.com",
		Status:  entity.Running,
		SSHUser: "ubuntu",
		SSHPort: 22,
	}
	client := &stubEnvironmentSSHClient{err: errors.New("dev-plane unavailable")}

	got := enrichWorkspacesWithSSHAccess(context.Background(), client, "user-1", []entity.Workspace{workspace})
	if diff := cmp.Diff([]entity.Workspace{workspace}, got); diff != "" {
		t.Fatalf("legacy workspace changed (-want +got): %s", diff)
	}
}

func TestEnrichWorkspacesWithSSHAccess_FallsBackWithoutPortBackedAccess(t *testing.T) {
	workspace := entity.Workspace{
		ID:      "env-1",
		DNS:     "legacy.example.com",
		Status:  entity.Running,
		SSHUser: "ubuntu",
		SSHPort: 22,
	}
	client := &stubEnvironmentSSHClient{environment: &devplanev1.Environment{
		Instance: &devplanev1.Instance{},
		SshAccess: []*devplanev1.SSHAccess{
			{UserId: "user-1", LinuxUser: "root"},
		},
	}}

	got := enrichWorkspacesWithSSHAccess(context.Background(), client, "user-1", []entity.Workspace{workspace})
	if diff := cmp.Diff([]entity.Workspace{workspace}, got); diff != "" {
		t.Fatalf("legacy workspace changed (-want +got): %s", diff)
	}
	if client.networkRequest != nil {
		t.Fatal("network info should not be fetched without port-backed access")
	}
}

func TestEnrichWorkspacesWithSSHAccess_MarksCertEligibleFromLabels(t *testing.T) {
	workspace := entity.Workspace{
		ID:     "env-1",
		Name:   "cert-env",
		Status: entity.Running,
	}
	client := &stubEnvironmentSSHClient{
		environment: &devplanev1.Environment{
			Labels:   map[string]string{"sshprovider": "certauth"},
			Instance: &devplanev1.Instance{SshHostname: "203.0.113.10", SshPort: 22, PublicIp: "203.0.113.10"},
			SshAccess: []*devplanev1.SSHAccess{
				{UserId: "user-1", LinuxUser: "ubuntu", PortId: "ssh-port"},
			},
		},
		networkInfo: &devplanev1.EnvironmentNetworkInfo{
			Ports: []*devplanev1.Port{
				{PortId: "ssh-port", Hostname: strPtr("skybridge.example.com"), PortNumber: 41234, ServerPort: 22},
			},
		},
	}

	got := enrichWorkspacesWithSSHAccess(context.Background(), client, "user-1", []entity.Workspace{workspace})
	if len(got) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(got))
	}
	if got[0].PortID != "ssh-port" {
		t.Errorf("PortID = %q, want %q", got[0].PortID, "ssh-port")
	}
	if !got[0].SSHCertEligible {
		t.Errorf("SSHCertEligible = false, want true (labels have sshprovider=certauth)")
	}
}

type slowSSHClient struct {
	latency   time.Duration
	deadlines []time.Time
	remaining []time.Duration
}

func (c *slowSSHClient) wait(ctx context.Context) error {
	deadline, _ := ctx.Deadline()
	c.deadlines = append(c.deadlines, deadline)
	c.remaining = append(c.remaining, time.Until(deadline))
	select {
	case <-time.After(c.latency):
		return nil
	case <-ctx.Done():
		return breverrors.WrapAndTrace(ctx.Err())
	}
}

func (c *slowSSHClient) GetEnvironment(
	ctx context.Context,
	req *connect.Request[devplanev1.GetEnvironmentRequest],
) (*connect.Response[devplanev1.GetEnvironmentResponse], error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(&devplanev1.GetEnvironmentResponse{
		Environment: &devplanev1.Environment{
			Instance: &devplanev1.Instance{},
			SshAccess: []*devplanev1.SSHAccess{
				{UserId: "user-1", LinuxUser: "ubuntu", PortId: req.Msg.GetEnvironmentId() + "-port"},
			},
		},
	}), nil
}

func (c *slowSSHClient) GetNetworkInfo(
	ctx context.Context,
	req *connect.Request[devplanev1.EnvironmentServiceGetNetworkInfoRequest],
) (*connect.Response[devplanev1.EnvironmentServiceGetNetworkInfoResponse], error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	return connect.NewResponse(&devplanev1.EnvironmentServiceGetNetworkInfoResponse{
		NetworkInfo: &devplanev1.EnvironmentNetworkInfo{
			Ports: []*devplanev1.Port{
				{PortId: req.Msg.GetEnvironmentId() + "-port", Hostname: strPtr("skybridge.example.com"), PortNumber: 41234},
			},
		},
	}), nil
}

func runningWorkspaces(ids ...string) []entity.Workspace {
	workspaces := make([]entity.Workspace, 0, len(ids))
	for _, id := range ids {
		workspaces = append(workspaces, entity.Workspace{ID: id, Status: entity.Running, SSHUser: "legacy", SSHPort: 22})
	}
	return workspaces
}

func TestEnrichWorkspacesWithSSHAccess_StartsAFreshDeadlinePerWorkspace(t *testing.T) {
	const latency = 20 * time.Millisecond
	client := &slowSSHClient{latency: latency}

	ctx, cancel := context.WithTimeout(context.Background(), sshAccessRefreshBudget)
	defer cancel()

	got := enrichWorkspacesWithSSHAccess(ctx, client, "user-1", runningWorkspaces("env-1", "env-2", "env-3"))

	if len(client.deadlines) != 6 {
		t.Fatalf("made %d RPCs, want 6", len(client.deadlines))
	}
	for i, remaining := range client.remaining {
		if remaining > sshAccessLookupTimeout {
			t.Errorf("RPC %d started with %s left, want at most the per-workspace %s", i, remaining, sshAccessLookupTimeout)
		}
	}
	for ws := 1; ws < 3; ws++ {
		prev, cur := client.deadlines[2*(ws-1)], client.deadlines[2*ws]
		if gap := cur.Sub(prev); gap < 2*latency {
			t.Errorf("workspace %d deadline is only %s after workspace %d's, want at least %s: workspaces are sharing one deadline",
				ws+1, gap, ws, 2*latency)
		}
	}
	for _, workspace := range got {
		if workspace.SSHHostname != "skybridge.example.com" {
			t.Errorf("workspace %s was not enriched: SSHHostname = %q", workspace.ID, workspace.SSHHostname)
		}
	}
}

func TestEnrichWorkspacesWithSSHAccess_LookupDeadlineNeverExceedsOverallBudget(t *testing.T) {
	client := &slowSSHClient{latency: time.Millisecond}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	budgetDeadline, _ := ctx.Deadline()

	enrichWorkspacesWithSSHAccess(ctx, client, "user-1", runningWorkspaces("env-1", "env-2"))

	if len(client.deadlines) == 0 {
		t.Fatal("no RPCs were made")
	}
	for i, deadline := range client.deadlines {
		if deadline.After(budgetDeadline) {
			t.Errorf("RPC %d deadline %s is past the overall budget %s", i, deadline, budgetDeadline)
		}
	}
}

func TestEnrichWorkspacesWithSSHAccess_StopsWhenOverallBudgetIsSpent(t *testing.T) {
	client := &slowSSHClient{latency: time.Millisecond}
	workspaces := runningWorkspaces("env-1", "env-2")
	want := runningWorkspaces("env-1", "env-2")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := enrichWorkspacesWithSSHAccess(ctx, client, "user-1", workspaces)

	if len(client.deadlines) != 0 {
		t.Errorf("made %d RPCs after the budget was spent, want 0", len(client.deadlines))
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("workspaces changed after the budget was spent (-want +got): %s", diff)
	}
}
