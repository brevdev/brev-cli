package heartbeat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/identity"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRunnerSendsHeartbeatAndUsesServerInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var captured client.HeartbeatParams
	clientStub := &stubClient{
		heartbeatFn: func(_ context.Context, params client.HeartbeatParams) (client.HeartbeatResult, error) {
			captured = params
			cancel()
			return client.HeartbeatResult{
				NextHeartbeatInterval: 45 * time.Second,
			}, nil
		},
	}

	var sleeps []time.Duration
	sleepFn := func(ctx context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		<-ctx.Done()
		return ctx.Err()
	}

	runner := Runner{
		Client: clientStub,
		Identity: identity.Identity{
			InstanceID:  "fn-1",
			DeviceToken: "token",
		},
		Cfg: HeartbeatConfig{
			BaseInterval: 30 * time.Second,
			MaxInterval:  time.Minute,
		},
		Log: zaptest.NewLogger(t),
		SampleUtilization: func(context.Context) (telemetry.UtilizationInfo, error) {
			return telemetry.UtilizationInfo{
				CPUPercent: 10,
			}, nil
		},
		Sleep: sleepFn,
		Now:   func() time.Time { return time.Unix(0, 0) },
	}

	err := runner.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, "fn-1", captured.BrevCloudNodeID)
	require.Equal(t, "token", captured.DeviceToken)
	require.NotNil(t, captured.Status)
	require.Equal(t, client.NodePhaseActive, captured.Status.Phase)
	require.Len(t, sleeps, 1)
	require.Equal(t, 45*time.Second, sleeps[0])
}

func TestRunnerBackoffOnFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callCount := 0
	clientStub := &stubClient{
		heartbeatFn: func(_ context.Context, _ client.HeartbeatParams) (client.HeartbeatResult, error) {
			callCount++
			if callCount < 3 {
				return client.HeartbeatResult{}, errors.New("temporary failure")
			}
			cancel()
			return client.HeartbeatResult{}, nil
		},
	}

	var sleeps []time.Duration
	sleepFn := func(ctx context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	runner := Runner{
		Client: clientStub,
		Identity: identity.Identity{
			InstanceID:  "fn",
			DeviceToken: "token",
		},
		Cfg: HeartbeatConfig{
			BaseInterval: time.Second,
			MaxInterval:  10 * time.Second,
		},
		Log: zaptest.NewLogger(t),
		SampleUtilization: func(context.Context) (telemetry.UtilizationInfo, error) {
			return telemetry.UtilizationInfo{}, nil
		},
		Sleep: sleepFn,
		Now:   time.Now,
	}

	err := runner.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, callCount)
	require.Equal(t, []time.Duration{2 * time.Second, 4 * time.Second, time.Second}, sleeps)
}

func TestRunnerAppliesStatusUpdates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	transition := time.Unix(100, 0)
	updates := make(chan client.HeartbeatStatus, 2)
	updates <- client.HeartbeatStatus{
		Phase:              client.NodePhaseError,
		Detail:             "tunnel missing",
		LastTransitionTime: &transition,
	}

	var captured []client.HeartbeatParams
	clientStub := &stubClient{
		heartbeatFn: func(_ context.Context, params client.HeartbeatParams) (client.HeartbeatResult, error) {
			captured = append(captured, params)
			cancel()
			return client.HeartbeatResult{}, nil
		},
	}

	runner := Runner{
		Client: clientStub,
		Identity: identity.Identity{
			InstanceID:  "inst-1",
			DeviceToken: "token",
		},
		Log: zaptest.NewLogger(t),
		SampleUtilization: func(context.Context) (telemetry.UtilizationInfo, error) {
			return telemetry.UtilizationInfo{}, nil
		},
		Sleep: func(ctx context.Context, _ time.Duration) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		},
		Now:           func() time.Time { return time.Unix(0, 0) },
		StatusUpdates: updates,
	}

	err := runner.Run(ctx)
	require.NoError(t, err)
	require.Len(t, captured, 1)
	require.NotNil(t, captured[0].Status)
	require.Equal(t, client.NodePhaseError, captured[0].Status.Phase)
	require.Equal(t, "tunnel missing", captured[0].Status.Detail)
	require.True(t, captured[0].Status.LastTransitionTime.Equal(transition))
}

func TestRunnerStopsOnUnauthenticated(t *testing.T) {
	ctx := context.Background()
	clientStub := &stubClient{
		heartbeatFn: func(context.Context, client.HeartbeatParams) (client.HeartbeatResult, error) {
			return client.HeartbeatResult{}, client.ErrUnauthenticated
		},
	}

	runner := Runner{
		Client: clientStub,
		Identity: identity.Identity{
			InstanceID:  "inst",
			DeviceToken: "token",
		},
		Log: zaptest.NewLogger(t),
		SampleUtilization: func(context.Context) (telemetry.UtilizationInfo, error) {
			return telemetry.UtilizationInfo{}, nil
		},
	}

	err := runner.Run(ctx)
	require.Error(t, err)
	require.True(t, errors.Is(err, client.ErrUnauthenticated))
}

type stubClient struct {
	heartbeatFn func(ctx context.Context, req client.HeartbeatParams) (client.HeartbeatResult, error)
}

func (s *stubClient) Register(_ context.Context, _ client.RegisterParams) (client.RegisterResult, error) {
	return client.RegisterResult{}, nil
}

func (s *stubClient) Heartbeat(ctx context.Context, req client.HeartbeatParams) (client.HeartbeatResult, error) {
	if s.heartbeatFn != nil {
		return s.heartbeatFn(ctx, req)
	}
	return client.HeartbeatResult{}, nil
}

func (s *stubClient) GetTunnelToken(_ context.Context, _ client.TunnelTokenParams) (client.TunnelTokenResult, error) {
	return client.TunnelTokenResult{}, nil
}
