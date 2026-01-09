package heartbeat

import (
	"context"
	"time"

	brevapiv2connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/brevapi/v2/brevapiv2connect"
	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/identity"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/brevdev/dev-plane/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultBaseInterval = 30 * time.Second
	defaultMaxInterval  = 5 * time.Minute
)

// HeartbeatConfig controls how frequently heartbeats run.
type HeartbeatConfig struct {
	BaseInterval time.Duration
	MaxInterval  time.Duration
}

// Runner drives the periodic heartbeat loop.
type Runner struct {
	Client   brevapiv2connect.BrevCloudAgentServiceClient
	Identity identity.Identity
	Cfg      HeartbeatConfig
	Log      *zap.Logger

	SampleUtilization func(context.Context) (telemetry.UtilizationInfo, error)
	Sleep             func(context.Context, time.Duration) error
	Now               func() time.Time

	DefaultStatus *brevapiv2.BrevCloudNodeStatus
	StatusUpdates <-chan *brevapiv2.BrevCloudNodeStatus
}

// Run executes the heartbeat loop until the context is canceled or an
// unrecoverable error occurs.
func (r *Runner) Run(ctx context.Context) error { //nolint:funlen // loop coordinates retries, telemetry, and server responses
	if err := r.validate(); err != nil {
		return errors.WrapAndTrace(err)
	}

	// sampleFn := r.SampleUtilization
	// if sampleFn == nil {
	// 	sampleFn = telemetry.SampleUtilization
	// }
	sleepFn := r.Sleep
	if sleepFn == nil {
		sleepFn = defaultSleep
	}
	nowFn := r.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	baseStatus := r.DefaultStatus
	if baseStatus == nil {
		baseStatus = &brevapiv2.BrevCloudNodeStatus{
			Phase: brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ACTIVE,
		}
	}
	currentStatus := cloneStatus(baseStatus)

	base := r.Cfg.BaseInterval
	if base <= 0 {
		base = defaultBaseInterval
	}
	maxInterval := r.Cfg.MaxInterval
	if maxInterval <= 0 {
		maxInterval = defaultMaxInterval
	}
	if maxInterval < base {
		maxInterval = base
	}

	nextDelay := time.Duration(0)
	currentBackoff := base

	for {
		if nextDelay > 0 {
			if err := sleepFn(ctx, nextDelay); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return errors.WrapAndTrace(err)
			}
		}

		// util, err := sampleFn(ctx)
		// if err != nil {
		// 	r.Log.Warn("failed to sample utilization", zap.Error(err))
		// 	currentBackoff = minDurationValue(currentBackoff*2, maxInterval)
		// 	nextDelay = currentBackoff
		// 	continue
		// }

		req := &brevapiv2.HeartbeatRequest{
			BrevCloudNodeId:       r.Identity.InstanceID,
			ObservedAt:            timestamppb.New(nowFn()),
			Status:                cloneStatus(currentStatus),
			DeviceFingerprintHash: r.Identity.DeviceFingerprintHash,
			HardwareFingerprint:   r.Identity.HardwareFingerprint,
		}

		if updated, ok := r.nextStatus(); ok {
			currentStatus = updated
			req.Status = cloneStatus(currentStatus)
		}

		connectReq := connect.NewRequest(req)
		connectReq.Header().Set("Authorization", client.BearerToken(r.Identity.DeviceToken))

		resp, err := r.Client.Heartbeat(ctx, connectReq)
		if err != nil {
			if classified := client.ClassifyError(err); classified != nil {
				if errors.Is(classified, client.ErrUnauthenticated) {
					return classified //nolint:wrapcheck // classification intentionally returned verbatim for caller handling
				}
				r.Log.Warn("heartbeat failed", zap.Error(classified))
				err = classified
			} else {
				r.Log.Warn("heartbeat failed", zap.Error(err))
			}
			currentBackoff = minDurationValue(currentBackoff*2, maxInterval)
			nextDelay = currentBackoff
			continue
		}

		currentBackoff = base
		nextDelay = base
		if interval := resp.Msg.GetNextHeartbeatInterval(); interval != nil {
			nextDelay = clampDuration(interval.AsDuration(), base, maxInterval)
		}
	}
}

func (r *Runner) validate() error {
	if r.Client == nil {
		return errors.Errorf("client is required")
	}
	if r.Identity.DeviceToken == "" {
		return errors.Errorf("device token is required")
	}
	if r.Identity.InstanceID == "" {
		return errors.Errorf("brevcloud node id is required")
	}
	if r.Log == nil {
		return errors.Errorf("logger is required")
	}
	return nil
}

func (r *Runner) nextStatus() (*brevapiv2.BrevCloudNodeStatus, bool) {
	if r.StatusUpdates == nil {
		return nil, false
	}

	var updated *brevapiv2.BrevCloudNodeStatus
	changed := false
	for {
		select {
		case status, ok := <-r.StatusUpdates:
			if !ok {
				r.StatusUpdates = nil
				return updated, changed
			}
			updated = normalizeStatus(status)
			changed = true
			continue
		default:
			return updated, changed
		}
	}
}

func normalizeStatus(status *brevapiv2.BrevCloudNodeStatus) *brevapiv2.BrevCloudNodeStatus {
	if status == nil {
		return nil
	}
	out := cloneStatus(status)
	if out.GetPhase() == brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_UNSPECIFIED {
		out.Phase = brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ACTIVE
	}
	return out
}

func cloneStatus(status *brevapiv2.BrevCloudNodeStatus) *brevapiv2.BrevCloudNodeStatus {
	if status == nil {
		return nil
	}
	out := *status
	if status.LastTransitionTime != nil {
		out.LastTransitionTime = timestamppb.New(status.LastTransitionTime.AsTime())
	}
	return &out
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return errors.WrapAndTrace(ctx.Err())
	case <-timer.C:
		return nil
	}
}

func minDurationValue(a, b time.Duration) time.Duration {
	if a <= b {
		return a
	}
	return b
}

func clampDuration(value, minInterval, maxInterval time.Duration) time.Duration {
	if value < minInterval {
		return minInterval
	}
	if value > maxInterval {
		return maxInterval
	}
	return value
}
