package refresh

import (
	"context"
	"log"
	"time"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
)

const workspaceSSHAccessTimeout = 10 * time.Second

type environmentGetter interface {
	GetEnvironment(context.Context, *connect.Request[devplanev1.GetEnvironmentRequest]) (*connect.Response[devplanev1.GetEnvironmentResponse], error)
	GetNetworkInfo(context.Context, *connect.Request[devplanev1.EnvironmentServiceGetNetworkInfoRequest]) (*connect.Response[devplanev1.EnvironmentServiceGetNetworkInfoResponse], error)
}

type sshAccessWorkspaceStore struct {
	RefreshStore
}

func (s sshAccessWorkspaceStore) GetContextWorkspaces() ([]entity.Workspace, error) {
	workspaces, err := s.RefreshStore.GetContextWorkspaces()
	if err != nil {
		return nil, err
	}

	user, err := s.GetCurrentUser()
	if err != nil {
		log.Printf("workspace SSH access: using legacy configuration (current user lookup failed): %v", err)
		return workspaces, nil
	}

	client := register.NewEnvironmentServiceClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
	ctx, cancel := context.WithTimeout(context.Background(), workspaceSSHAccessTimeout)
	defer cancel()

	return enrichWorkspacesWithSSHAccess(ctx, client, user.ID, workspaces), nil
}

func enrichWorkspacesWithSSHAccess(ctx context.Context, client environmentGetter, userID string, workspaces []entity.Workspace) []entity.Workspace {
	for i := range workspaces {
		if workspaces[i].Status != entity.Running {
			continue
		}

		res, err := client.GetEnvironment(ctx, connect.NewRequest(&devplanev1.GetEnvironmentRequest{
			EnvironmentId: workspaces[i].ID,
			AttachedDataOptions: &devplanev1.GetEnvironmentAttachedDataOptions{
				Instance:  true,
				SshAccess: true,
			},
		}))
		if err != nil {
			log.Printf("workspace SSH access: using legacy configuration for %s: %v", workspaces[i].ID, err)
			continue
		}
		if res == nil || res.Msg == nil {
			log.Printf("workspace SSH access: using legacy configuration for %s: empty response", workspaces[i].ID)
			continue
		}

		environment := res.Msg.GetEnvironment()
		access := getPortBackedSSHAccess(environment, userID)
		if access == nil {
			log.Printf("workspace SSH access: using legacy configuration for %s (no port-backed access for current user)", workspaces[i].ID)
			continue
		}

		networkRes, err := client.GetNetworkInfo(ctx, connect.NewRequest(&devplanev1.EnvironmentServiceGetNetworkInfoRequest{
			EnvironmentId: workspaces[i].ID,
		}))
		if err != nil {
			log.Printf("workspace SSH access: using legacy configuration for %s (network lookup failed): %v", workspaces[i].ID, err)
			continue
		}
		if networkRes == nil || networkRes.Msg == nil {
			log.Printf("workspace SSH access: using legacy configuration for %s (empty network response)", workspaces[i].ID)
			continue
		}

		port := getSSHAccessPort(networkRes.Msg.GetNetworkInfo(), access.GetPortId())
		if port == nil {
			log.Printf("workspace SSH access: using legacy configuration for %s (port %s not found)", workspaces[i].ID, access.GetPortId())
			continue
		}

		workspaces[i] = applyEnvironmentSSHAccess(workspaces[i], environment.GetInstance(), access, port)
		log.Printf(
			"workspace SSH access: resolved %s as %s@%s:%d from port %s",
			workspaces[i].ID,
			workspaces[i].SSHUser,
			workspaces[i].SSHHostname,
			workspaces[i].SSHPort,
			access.GetPortId(),
		)
	}
	return workspaces
}

func applyEnvironmentSSHAccess(
	workspace entity.Workspace,
	instance *devplanev1.Instance,
	access *devplanev1.SSHAccess,
	port *devplanev1.Port,
) entity.Workspace {
	if access == nil || port == nil || port.GetHostname() == "" || port.GetPortNumber() == 0 {
		return workspace
	}

	workspace.SSHHostname = port.GetHostname()
	workspace.SSHPort = int(port.GetPortNumber())
	workspace.SSHUser = access.GetLinuxUser()
	workspace.SSHProxyHostname = ""

	if providerHostname := getProviderSSHHostname(instance); providerHostname != "" {
		workspace.HostSSHHostname = providerHostname
		workspace.HostSSHProxyHostname = ""
	}
	return workspace
}

func getPortBackedSSHAccess(environment *devplanev1.Environment, userID string) *devplanev1.SSHAccess {
	for _, access := range environment.GetSshAccess() {
		// A PortId identifies access through the resolved Skybridge endpoint.
		// Legacy Cloudflare access has no PortId and keeps using the existing proxy config.
		if access.GetUserId() == userID && access.GetPortId() != "" {
			return access
		}
	}
	return nil
}

func getSSHAccessPort(networkInfo *devplanev1.EnvironmentNetworkInfo, portID string) *devplanev1.Port {
	for _, port := range networkInfo.GetPorts() {
		if port.GetPortId() == portID {
			return port
		}
	}
	return nil
}

func getProviderSSHHostname(instance *devplanev1.Instance) string {
	if instance.GetPublicIp() != "" {
		return instance.GetPublicIp()
	}
	if instance.GetPublicDns() != "" && instance.GetPublicDns() != instance.GetSshHostname() {
		return instance.GetPublicDns()
	}
	if instance.GetHostname() != "" && instance.GetHostname() != instance.GetSshHostname() {
		return instance.GetHostname()
	}
	return ""
}
