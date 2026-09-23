package refresh

import (
	"context"
	"errors"
	"testing"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"

	"github.com/brevdev/brev-cli/pkg/entity"
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
