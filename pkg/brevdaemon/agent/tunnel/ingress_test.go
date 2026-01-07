package tunnel

import (
	"context"
	"testing"
	"time"

	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/identity"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/brevdev/dev-plane/pkg/brevcloud/appaccess"
	"github.com/brevdev/dev-plane/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestDetectInstanceTypeFromHardware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hw   telemetry.HardwareInfo
		want appaccess.InstanceType
	}{
		{
			name: "machine model DGX",
			hw:   telemetry.HardwareInfo{MachineModel: "DGX GH200"},
			want: appaccess.InstanceTypeDGXSpark,
		},
		{
			name: "gpu model DGX",
			hw: telemetry.HardwareInfo{
				GPUs: []telemetry.GPUInfo{{Model: "DGX A100"}},
			},
			want: appaccess.InstanceTypeDGXSpark,
		},
		{
			name: "non DGX",
			hw:   telemetry.HardwareInfo{MachineModel: "c2-standard"},
			want: appaccess.InstanceTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := detectInstanceTypeFromHardware(tt.hw)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildAppIngressesSparkHealthy(t *testing.T) {
	t.Parallel()

	cfg := appaccess.DefaultConfig()
	ingresses := buildAppIngresses(
		context.Background(),
		cfg,
		nil,
		successHTTPProbe,
		stubSystemd(true, true),
		500*time.Millisecond,
		appaccess.InstanceTypeDGXSpark,
	)

	require.Len(t, ingresses, 2)
	requireAppIngress(t, ingresses, string(appaccess.AppIDDGXDashboard), 11000)
	requireAppIngress(t, ingresses, string(appaccess.AppIDJupyter), 8888)
}

func TestBuildAppIngressesNonSpark(t *testing.T) {
	t.Parallel()

	cfg := appaccess.DefaultConfig()
	ingresses := buildAppIngresses(context.Background(), cfg, nil, successHTTPProbe, stubSystemd(true, true), 500*time.Millisecond, appaccess.InstanceTypeUnknown)
	require.Nil(t, ingresses)
}

func TestBuildAppIngressesDashboardInactiveSkips(t *testing.T) {
	t.Parallel()

	cfg := appaccess.DefaultConfig()
	ingresses := buildAppIngresses(context.Background(), cfg, nil, successHTTPProbe, stubSystemd(false, true), 500*time.Millisecond, appaccess.InstanceTypeDGXSpark)

	require.Len(t, ingresses, 1)
	requireAppIngress(t, ingresses, string(appaccess.AppIDJupyter), 8888)
}

func TestBuildAppIngressesDashboardFallsBackWhenSystemdUnavailable(t *testing.T) {
	t.Parallel()

	cfg := appaccess.DefaultConfig()
	ingresses := buildAppIngresses(context.Background(), cfg, nil, successHTTPProbe, stubSystemd(false, false), 500*time.Millisecond, appaccess.InstanceTypeDGXSpark)

	require.Len(t, ingresses, 2)
	requireAppIngress(t, ingresses, string(appaccess.AppIDDGXDashboard), 11000)
	requireAppIngress(t, ingresses, string(appaccess.AppIDJupyter), 8888)
}

func TestBuildAppIngressesProbeFailureSkips(t *testing.T) {
	t.Parallel()

	cfg := appaccess.DefaultConfig()
	ingresses := buildAppIngresses(context.Background(), cfg, nil, failingHTTPProbe, stubSystemd(false, false), 500*time.Millisecond, appaccess.InstanceTypeDGXSpark)
	require.Nil(t, ingresses)
}

func stubSystemd(active bool, supported bool) systemdStatusFunc {
	return func(context.Context, string, time.Duration) (bool, bool, error) {
		return active, supported, nil
	}
}

func successHTTPProbe(context.Context, string, int, string, time.Duration) error {
	return nil
}

func failingHTTPProbe(context.Context, string, int, string, time.Duration) error {
	return errors.New("probe failed")
}

func requireAppIngress(t *testing.T, ingresses []client.AppIngress, appID string, port int) {
	t.Helper()
	for _, ingress := range ingresses {
		if ingress.AppID == appID {
			require.Equal(t, port, ingress.LocalPort)
			require.Equal(t, "https", ingress.Protocol)
			require.Equal(t, "/", ingress.PathPrefix)
			require.Equal(t, appID, ingress.HostnamePrefix)
			require.True(t, ingress.ForceHTTPS)
			return
		}
	}
	require.Fail(t, "expected ingress not found", "app_id=%s", appID)
}

func TestManagerRequestsSSHWithoutAppIngressesForNonSpark(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientStub := &stubAgentClient{
		resp: client.TunnelTokenResult{Token: "token"},
	}
	manager := Manager{
		Client:   clientStub,
		Identity: identity.Identity{InstanceID: "brev-node", DeviceToken: "device-token"},
		Cfg:      TunnelConfig{SSHPort: 22},
		Log:      zap.NewNop(),
		DetectHardware: func(context.Context) (telemetry.HardwareInfo, error) {
			return telemetry.HardwareInfo{MachineModel: "c2-standard"}, nil
		},
		HTTPProbe:      successHTTPProbe,
		SystemdStatus:  stubSystemd(true, true),
		CommandFactory: func(context.Context, string, ...string) Command { return stubCommand{} },
		ProbeTimeout:   500 * time.Millisecond,
		AppConfig:      appaccess.DefaultConfig(),
		Sleep:          func(context.Context, time.Duration) error { return nil },
		Probe:          defaultIngressProbe,
	}

	require.NoError(t, manager.Start(ctx))
	require.Len(t, clientStub.lastReq.Ports, 1)
	require.Equal(t, 22, clientStub.lastReq.Ports[0].LocalPort)
	require.Empty(t, clientStub.lastReq.AppIngresses)
}

func TestManagerIncludesAppIngressesForSparkWhenProbesPass(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clientStub := &stubAgentClient{
		resp: client.TunnelTokenResult{Token: "token"},
	}
	manager := Manager{
		Client:   clientStub,
		Identity: identity.Identity{InstanceID: "brev-node", DeviceToken: "device-token"},
		Cfg:      TunnelConfig{SSHPort: 22},
		Log:      zap.NewNop(),
		DetectHardware: func(context.Context) (telemetry.HardwareInfo, error) {
			return telemetry.HardwareInfo{MachineModel: "DGX Station"}, nil
		},
		HTTPProbe:      successHTTPProbe,
		SystemdStatus:  stubSystemd(true, true),
		CommandFactory: func(context.Context, string, ...string) Command { return stubCommand{} },
		ProbeTimeout:   500 * time.Millisecond,
		AppConfig:      appaccess.DefaultConfig(),
		Sleep:          func(context.Context, time.Duration) error { return nil },
		Probe:          defaultIngressProbe,
	}

	require.NoError(t, manager.Start(ctx))
	require.Len(t, clientStub.lastReq.Ports, 1)
	require.Len(t, clientStub.lastReq.AppIngresses, 2)
	requireAppIngressParams(t, clientStub.lastReq.AppIngresses, string(appaccess.AppIDDGXDashboard), 11000)
	requireAppIngressParams(t, clientStub.lastReq.AppIngresses, string(appaccess.AppIDJupyter), 8888)
}

func requireAppIngressParams(t *testing.T, ingresses []client.AppIngress, appID string, port int) {
	t.Helper()
	for _, ingress := range ingresses {
		if ingress.AppID == appID {
			require.Equal(t, port, ingress.LocalPort)
			require.Equal(t, appID, ingress.HostnamePrefix)
			require.Equal(t, "https", ingress.Protocol)
			require.True(t, ingress.ForceHTTPS)
			return
		}
	}
	require.Fail(t, "expected app ingress missing", "app_id=%s", appID)
}

type stubAgentClient struct {
	lastReq client.TunnelTokenParams
	resp    client.TunnelTokenResult
}

func (s *stubAgentClient) Register(context.Context, client.RegisterParams) (client.RegisterResult, error) {
	return client.RegisterResult{}, nil
}

func (s *stubAgentClient) Heartbeat(context.Context, client.HeartbeatParams) (client.HeartbeatResult, error) {
	return client.HeartbeatResult{}, nil
}

func (s *stubAgentClient) GetTunnelToken(_ context.Context, req client.TunnelTokenParams) (client.TunnelTokenResult, error) {
	s.lastReq = req
	return s.resp, nil
}

type stubCommand struct{}

func (stubCommand) Start() error                    { return nil }
func (stubCommand) Wait() error                     { return nil }
func (stubCommand) SetEnv([]string)                 {}
func (stubCommand) CombinedOutput() ([]byte, error) { return nil, nil }
