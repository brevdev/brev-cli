package agent

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	agentconfig "github.com/brevdev/brev-cli/pkg/brevdaemon/agent/config"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/heartbeat"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/identity"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestNewAgentBuildsDependencies(t *testing.T) {
	origClientFactory := newBrevCloudAgentClient
	origDetect := detectHardware
	origEnsure := ensureIdentity

	t.Cleanup(func() {
		newBrevCloudAgentClient = origClientFactory
		detectHardware = origDetect
		ensureIdentity = origEnsure
	})

	newBrevCloudAgentClient = func(agentconfig.Config, ...client.Option) (client.BrevCloudAgentClient, error) {
		return &stubBrevCloudClient{}, nil
	}

	var detectCalled bool
	detectHardware = func(context.Context) (telemetry.HardwareInfo, error) {
		detectCalled = true
		return telemetry.HardwareInfo{CPUCount: 8}, nil
	}

	var ensureCalled bool
	ensureIdentity = func(_ context.Context, _ agentconfig.Config, _ client.BrevCloudAgentClient, _ *identity.IdentityStore, _ telemetry.HardwareInfo, _ *zap.Logger) (identity.Identity, error) {
		ensureCalled = true
		return identity.Identity{
			InstanceID:  "inst-1",
			DeviceToken: "token-1",
		}, nil
	}

	dir := t.TempDir()
	cfg := agentconfig.Config{
		BrevCloudAgentURL: "https://example.dev/v1",
		StateDir:          dir,
		DeviceTokenPath:   filepath.Join(dir, "device_token"),
		RegistrationToken: "reg-token",
		EnableTunnel:      true,
		TunnelSSHPort:     2022,
		HeartbeatInterval: time.Second,
	}

	agentIface, err := NewAgent(cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.True(t, detectCalled)
	require.True(t, ensureCalled)

	a, ok := agentIface.(*agent)
	require.True(t, ok)
	require.NotNil(t, a.heartbeat)
	require.NotNil(t, a.tunnel)
	require.NotNil(t, a.statusReporter)
	require.NotNil(t, a.statusUpdates)

	runner, ok := a.heartbeat.(*heartbeat.Runner)
	require.True(t, ok)
	require.NotNil(t, runner.StatusUpdates)
	require.NotNil(t, runner.DefaultStatus)
}

func TestAgentRunPropagatesHeartbeatError(t *testing.T) {
	hbErr := errors.New("heartbeat failed")
	hb := &stubRunner{
		runFn: func(context.Context) error {
			return hbErr
		},
	}

	a := &agent{
		log:       zaptest.NewLogger(t),
		heartbeat: hb,
	}

	err := a.Run(context.Background())
	require.ErrorIs(t, err, hbErr)
}

func TestAgentRunStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hb := &stubRunner{
		runFn: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
	}
	tm := &stubTunnel{
		startFn: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
	}

	a := &agent{
		log:       zaptest.NewLogger(t),
		heartbeat: hb,
		tunnel:    tm,
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := a.Run(ctx)
	require.NoError(t, err)
}

func TestAgentRunIgnoresTunnelError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hb := &stubRunner{
		runFn: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
	}

	var startCalls int
	tm := &stubTunnel{
		startFn: func(ctx context.Context) error {
			startCalls++
			if startCalls == 1 {
				return errors.New("boom")
			}
			<-ctx.Done()
			return ctx.Err()
		},
	}

	a := &agent{
		log:       zaptest.NewLogger(t),
		heartbeat: hb,
		tunnel:    tm,
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := a.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, startCalls)
}

type stubBrevCloudClient struct{}

func (s *stubBrevCloudClient) Register(context.Context, client.RegisterParams) (client.RegisterResult, error) {
	return client.RegisterResult{}, nil
}

func (s *stubBrevCloudClient) Heartbeat(context.Context, client.HeartbeatParams) (client.HeartbeatResult, error) {
	return client.HeartbeatResult{}, nil
}

func (s *stubBrevCloudClient) GetTunnelToken(context.Context, client.TunnelTokenParams) (client.TunnelTokenResult, error) {
	return client.TunnelTokenResult{}, nil
}

type stubRunner struct {
	runFn func(context.Context) error
}

func (s *stubRunner) Run(ctx context.Context) error {
	if s.runFn != nil {
		return s.runFn(ctx)
	}
	return nil
}

type stubTunnel struct {
	startFn func(context.Context) error
}

func (s *stubTunnel) Start(ctx context.Context) error {
	if s.startFn != nil {
		return s.startFn(ctx)
	}
	return nil
}
