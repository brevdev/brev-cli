package heartbeat

import (
	"context"
	"time"

	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/identity"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/brevdev/dev-plane/pkg/errors"
	"go.uber.org/zap"
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
	Client   client.BrevCloudAgentClient
	Identity identity.Identity
	Cfg      HeartbeatConfig
	Log      *zap.Logger

	SampleUtilization func(context.Context) (telemetry.UtilizationInfo, error)
	Sleep             func(context.Context, time.Duration) error
	Now               func() time.Time

	DefaultStatus *client.HeartbeatStatus
	StatusUpdates <-chan client.HeartbeatStatus
}

// Run executes the heartbeat loop until the context is canceled or an
// unrecoverable error occurs.
func (r *Runner) Run(ctx context.Context) error { //nolint:gocyclo,funlen // loop coordinates retries, telemetry, and server responses
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
		baseStatus = &client.HeartbeatStatus{
			Phase: client.NodePhaseActive,
		}
	}
	currentStatus := cloneStatus(*baseStatus)

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

		params := client.HeartbeatParams{
			BrevCloudNodeID: r.Identity.InstanceID,
			DeviceToken:     r.Identity.DeviceToken,
			ObservedAt:      nowFn(),
			// Utilization:           util.ToClient(),
			AgentVersion:          "", // set by orchestrator later
			Status:                currentStatusPtr(currentStatus),
			DeviceFingerprintHash: r.Identity.DeviceFingerprintHash,
			HardwareFingerprint:   r.Identity.HardwareFingerprint,
		}

		if updated, ok := r.nextStatus(); ok {
			currentStatus = updated
			params.Status = currentStatusPtr(currentStatus)
		}

		res, err := r.Client.Heartbeat(ctx, params)
		if err != nil {
			if errors.Is(err, client.ErrUnauthenticated) {
				return errors.WrapAndTrace(err)
			}
			r.Log.Warn("heartbeat failed", zap.Error(err))
			currentBackoff = minDurationValue(currentBackoff*2, maxInterval)
			nextDelay = currentBackoff
			continue
		}

		currentBackoff = base
		nextDelay = base
		if res.NextHeartbeatInterval > 0 {
			nextDelay = clampDuration(res.NextHeartbeatInterval, base, maxInterval)
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

func (r *Runner) nextStatus() (client.HeartbeatStatus, bool) {
	if r.StatusUpdates == nil {
		return client.HeartbeatStatus{}, false
	}

	var updated client.HeartbeatStatus
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

func normalizeStatus(status client.HeartbeatStatus) client.HeartbeatStatus {
	if status.Phase == client.NodePhaseUnspecified {
		status.Phase = client.NodePhaseActive
	}
	return cloneStatus(status)
}

func cloneStatus(status client.HeartbeatStatus) client.HeartbeatStatus {
	out := status
	if status.LastTransitionTime != nil {
		t := *status.LastTransitionTime
		out.LastTransitionTime = &t
	}
	return out
}

func currentStatusPtr(status client.HeartbeatStatus) *client.HeartbeatStatus {
	s := cloneStatus(status)
	return &s
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
