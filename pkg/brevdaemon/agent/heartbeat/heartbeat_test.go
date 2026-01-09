package heartbeat

import (
	"context"
	stderrs "errors"
	"testing"
	"time"

	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/identity"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRunnerSendsHeartbeatAndUsesServerInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var captured *connect.Request[brevapiv2.HeartbeatRequest]
	clientStub := &stubClient{
		heartbeatFn: func(_ context.Context, req *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error) {
			captured = req
			cancel()
			return connect.NewResponse(&brevapiv2.HeartbeatResponse{
				NextHeartbeatInterval: durationpb.New(45 * time.Second),
			}), nil
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
		Log:   zaptest.NewLogger(t),
		Sleep: sleepFn,
		Now:   func() time.Time { return time.Unix(0, 0) },
	}

	err := runner.Run(ctx)
	require.NoError(t, err)
	require.NotNil(t, captured)
	require.Equal(t, "fn-1", captured.Msg.GetBrevCloudNodeId())
	require.Equal(t, brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ACTIVE, captured.Msg.GetStatus().GetPhase())
	require.Len(t, sleeps, 1)
	require.Equal(t, 45*time.Second, sleeps[0])
}

func TestRunnerBackoffOnFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callCount := 0
	clientStub := &stubClient{
		heartbeatFn: func(_ context.Context, _ *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error) {
			callCount++
			if callCount < 3 {
				return nil, stderrs.New("temporary failure")
			}
			cancel()
			return connect.NewResponse(&brevapiv2.HeartbeatResponse{}), nil
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
		Log:   zaptest.NewLogger(t),
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
	updates := make(chan *brevapiv2.BrevCloudNodeStatus, 2)
	updates <- &brevapiv2.BrevCloudNodeStatus{
		Phase:              brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ERROR,
		Detail:             "tunnel missing",
		LastTransitionTime: timestamppb.New(transition),
	}

	var captured []*connect.Request[brevapiv2.HeartbeatRequest]
	clientStub := &stubClient{
		heartbeatFn: func(_ context.Context, req *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error) {
			captured = append(captured, req)
			cancel()
			return connect.NewResponse(&brevapiv2.HeartbeatResponse{}), nil
		},
	}

	runner := Runner{
		Client: clientStub,
		Identity: identity.Identity{
			InstanceID:  "inst-1",
			DeviceToken: "token",
		},
		Log: zaptest.NewLogger(t),
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
	require.NotNil(t, captured[0].Msg.GetStatus())
	require.Equal(t, brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ERROR, captured[0].Msg.GetStatus().GetPhase())
	require.Equal(t, "tunnel missing", captured[0].Msg.GetStatus().GetDetail())
	require.True(t, captured[0].Msg.GetStatus().GetLastTransitionTime().AsTime().Equal(transition))
}

func TestRunnerStopsOnUnauthenticated(t *testing.T) {
	ctx := context.Background()
	clientStub := &stubClient{
		heartbeatFn: func(context.Context, *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error) {
			return nil, connect.NewError(connect.CodeUnauthenticated, stderrs.New("invalid token"))
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
	require.True(t, stderrs.Is(err, client.ErrUnauthenticated))
}

type stubClient struct {
	heartbeatFn func(ctx context.Context, req *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error)
}

func (s *stubClient) Register(_ context.Context, _ *connect.Request[brevapiv2.RegisterRequest]) (*connect.Response[brevapiv2.RegisterResponse], error) {
	return connect.NewResponse(&brevapiv2.RegisterResponse{}), nil
}

func (s *stubClient) Heartbeat(ctx context.Context, req *connect.Request[brevapiv2.HeartbeatRequest]) (*connect.Response[brevapiv2.HeartbeatResponse], error) {
	if s.heartbeatFn != nil {
		return s.heartbeatFn(ctx, req)
	}
	return connect.NewResponse(&brevapiv2.HeartbeatResponse{}), nil
}

func (s *stubClient) GetTunnelToken(_ context.Context, _ *connect.Request[brevapiv2.GetTunnelTokenRequest]) (*connect.Response[brevapiv2.GetTunnelTokenResponse], error) {
	return connect.NewResponse(&brevapiv2.GetTunnelTokenResponse{}), nil
}
