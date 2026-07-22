package refresh

import (
	"context"
	"fmt"
	"log"
	"time"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const sshAccessLookupTimeout = 10 * time.Second

type environmentSSHClient interface {
	GetEnvironment(context.Context, *connect.Request[devplanev1.GetEnvironmentRequest]) (*connect.Response[devplanev1.GetEnvironmentResponse], error)
	GetNetworkInfo(context.Context, *connect.Request[devplanev1.EnvironmentServiceGetNetworkInfoRequest]) (*connect.Response[devplanev1.EnvironmentServiceGetNetworkInfoResponse], error)
}

type workspaceSSHStore struct {
	RefreshStore
}

func (s workspaceSSHStore) GetContextWorkspaces() ([]entity.Workspace, error) {
	workspaces, err := s.RefreshStore.GetContextWorkspaces()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	user, err := s.GetCurrentUser()
	if err != nil {
		log.Printf("workspace SSH access: using legacy configuration (current user lookup failed): %v", err)
		return workspaces, nil
	}

	client := register.NewEnvironmentServiceClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	ctx, cancel := context.WithTimeout(context.Background(), sshAccessLookupTimeout)
	defer cancel()

	return enrichWorkspacesWithSSHAccess(ctx, client, user.ID, workspaces), nil
}

func enrichWorkspacesWithSSHAccess(ctx context.Context, client environmentSSHClient, userID string, workspaces []entity.Workspace) []entity.Workspace {
	for i := range workspaces {
		if workspaces[i].Status != entity.Running {
			continue
		}

		workspace, err := resolveWorkspaceSSH(ctx, client, userID, workspaces[i])
		if err != nil {
			log.Printf("workspace SSH access: using legacy configuration for %s: %v", workspaces[i].ID, err)
			continue
		}
		workspaces[i] = workspace
	}
	return workspaces
}

func resolveWorkspaceSSH(
	ctx context.Context,
	client environmentSSHClient,
	userID string,
	workspace entity.Workspace,
) (entity.Workspace, error) {
	environmentRes, err := client.GetEnvironment(ctx, connect.NewRequest(&devplanev1.GetEnvironmentRequest{
		EnvironmentId: workspace.ID,
		AttachedDataOptions: &devplanev1.GetEnvironmentAttachedDataOptions{
			Instance:  true,
			SshAccess: true,
		},
	}))
	if err != nil {
		return workspace, fmt.Errorf("get environment: %w", err)
	}
	if environmentRes == nil || environmentRes.Msg == nil || environmentRes.Msg.GetEnvironment() == nil {
		return workspace, fmt.Errorf("get environment: empty response")
	}

	// Use the ssh access information to determine the SSH target, rather than the public IP, DNS, or other information
	// returned by the initial workspace query.
	environment := environmentRes.Msg.GetEnvironment()
	access := findUserSSHAccess(environment.GetSshAccess(), userID)
	if access == nil {
		return workspace, nil
	}
	if access.GetLinuxUser() == "" {
		return workspace, fmt.Errorf("SSH access has no Linux user")
	}

	// Using the port ID, we can lookup the network information to get the hostname and port number of the SSH target.
	networkRes, err := client.GetNetworkInfo(ctx, connect.NewRequest(&devplanev1.EnvironmentServiceGetNetworkInfoRequest{
		EnvironmentId: workspace.ID,
	}))
	if err != nil {
		return workspace, fmt.Errorf("get network info: %w", err)
	}
	if networkRes == nil || networkRes.Msg == nil {
		return workspace, fmt.Errorf("get network info: empty response")
	}

	port := findNetworkPort(networkRes.Msg.GetNetworkInfo(), access.GetPortId())
	if port == nil {
		return workspace, fmt.Errorf("SSH access port %q not found", access.GetPortId())
	}
	if port.GetHostname() == "" || port.GetPortNumber() == 0 {
		return workspace, fmt.Errorf("SSH access port %q has no endpoint", access.GetPortId())
	}

	// Honor the SSH access information as the source of truth for the SSH target.
	workspace.SSHHostname = port.GetHostname()
	workspace.SSHPort = int(port.GetPortNumber())
	workspace.SSHUser = access.GetLinuxUser()
	workspace.SSHProxyHostname = ""

	// To support the "--host" fallback, preserve the legacy hostname information returned by the initial workspace query.
	if providerHostname := providerSSHHostname(environment.GetInstance(), port.GetHostname()); providerHostname != "" {
		workspace.HostSSHHostname = providerHostname
		workspace.HostSSHProxyHostname = ""
	}
	return workspace, nil
}

func findUserSSHAccess(accesses []*devplanev1.SSHAccess, userID string) *devplanev1.SSHAccess {
	// Technically it is possible that multiple SSHAccess entries exist for a single user+environment. This is typically only
	// for external nodes, which allow for multiple sshd processes to be targeted. For the normal environment flow, the below
	// is a best-effort approach to find the *first* applicable entry.
	for _, access := range accesses {
		if access.GetUserId() == userID && access.GetPortId() != "" {
			return access
		}
	}
	return nil
}

func findNetworkPort(networkInfo *devplanev1.EnvironmentNetworkInfo, portID string) *devplanev1.Port {
	for _, port := range networkInfo.GetPorts() {
		if port.GetPortId() == portID {
			return port
		}
	}
	return nil
}

func providerSSHHostname(instance *devplanev1.Instance, workloadHostname string) string {
	if instance == nil {
		return ""
	}
	for _, hostname := range []string{instance.GetPublicIp(), instance.GetPublicDns(), instance.GetHostname()} {
		if hostname != "" && hostname != workloadHostname {
			return hostname
		}
	}
	return ""
}
