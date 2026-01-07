package tunnel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/health"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/identity"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestManagerConfiguresCloudflaredService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requested := false
	stubClient := &stubClient{
		getTunnelFn: func(context.Context, client.TunnelTokenParams) (client.TunnelTokenResult, error) {
			requested = true
			return client.TunnelTokenResult{
				Token:    "token",
				Endpoint: "endpoint",
				TTL:      time.Minute,
			}, nil
		},
	}

	var executed []struct {
		name string
		args []string
	}
	reporter := &fakeReporter{}
	manager := Manager{
		Client: stubClient,
		Identity: identity.Identity{
			InstanceID:  "inst",
			DeviceToken: "device",
		},
		Cfg:    TunnelConfig{SSHPort: 22},
		Log:    zaptest.NewLogger(t),
		Health: reporter,
		CommandFactory: func(_ context.Context, name string, args ...string) Command {
			executed = append(executed, struct {
				name string
				args []string
			}{name: name, args: args})
			return &fakeCmd{
				startFn: func() error {
					return nil
				},
				waitFn: func() error { return nil },
			}
		},
		jitter: func(d time.Duration) time.Duration { return d },
	}

	err := manager.Start(ctx)
	require.NoError(t, err)
	require.True(t, requested)
	require.Len(t, executed, 2)
	require.Equal(t, "sudo", executed[0].name)
	require.Equal(t, []string{"cloudflared", "service", "install", "token"}, executed[0].args)
	require.Equal(t, "sudo", executed[1].name)
	require.Equal(t, []string{"systemctl", "restart", "cloudflared.service"}, executed[1].args)
	require.NotEmpty(t, reporter.actives)
	require.Contains(t, reporter.actives[0], "cloudflared service configured")
}

func TestManagerRetriesOnConfigureError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var sleepCalls int
	reporter := &fakeReporter{}

	stubClient := &stubClient{
		getTunnelFn: func(context.Context, client.TunnelTokenParams) (client.TunnelTokenResult, error) {
			return client.TunnelTokenResult{
				Token:    "token",
				Endpoint: "endpoint",
			}, nil
		},
	}

	commandCalls := 0

	manager := Manager{
		Client: stubClient,
		Health: reporter,
		jitter: func(d time.Duration) time.Duration { return d },
		Log:    zaptest.NewLogger(t),
		Sleep: func(_ context.Context, _ time.Duration) error {
			sleepCalls++
			return nil
		},
		Identity: identity.Identity{
			InstanceID:  "inst",
			DeviceToken: "device",
		},
		CommandFactory: func(cmdCtx context.Context, _ string, _ ...string) Command {
			commandCalls++
			if commandCalls == 1 {
				return &fakeCmd{
					startFn: func() error { return errors.New("boom") },
					waitFn:  func() error { return nil },
				}
			}
			return &fakeCmd{
				startFn: func() error { return nil },
				waitFn: func() error {
					cancel()
					<-cmdCtx.Done()
					return context.Canceled
				},
			}
		},
	}

	err := manager.Start(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, commandCalls)
	require.GreaterOrEqual(t, sleepCalls, 1)
	require.NotEmpty(t, reporter.errors)
	require.Contains(t, reporter.errors[0], "failed to configure cloudflared service")
}

type fakeCmd struct {
	startFn func() error
	waitFn  func() error
	env     []string
}

func (f *fakeCmd) Start() error {
	if f.startFn != nil {
		return f.startFn()
	}
	return nil
}

func (f *fakeCmd) Wait() error {
	if f.waitFn != nil {
		return f.waitFn()
	}
	return nil
}

func (f *fakeCmd) CombinedOutput() ([]byte, error) {
	if err := f.Start(); err != nil {
		return nil, err
	}
	return nil, f.Wait()
}

func (f *fakeCmd) SetEnv(env []string) {
	f.env = env
}

type fakeReporter struct {
	errors  []string
	actives []string
}

func (f *fakeReporter) MarkActive(detail string) health.Status {
	f.actives = append(f.actives, detail)
	return health.Status{}
}

func (f *fakeReporter) MarkError(detail string) health.Status {
	f.errors = append(f.errors, detail)
	return health.Status{}
}

type stubClient struct {
	getTunnelFn func(context.Context, client.TunnelTokenParams) (client.TunnelTokenResult, error)
}

func (s *stubClient) Register(context.Context, client.RegisterParams) (client.RegisterResult, error) {
	return client.RegisterResult{}, nil
}

func (s *stubClient) Heartbeat(context.Context, client.HeartbeatParams) (client.HeartbeatResult, error) {
	return client.HeartbeatResult{}, nil
}

func (s *stubClient) GetTunnelToken(ctx context.Context, params client.TunnelTokenParams) (client.TunnelTokenResult, error) {
	if s.getTunnelFn != nil {
		return s.getTunnelFn(ctx, params)
	}
	return client.TunnelTokenResult{}, nil
}
