package health

import (
	"sync"
	"time"

	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
)

// Status mirrors the heartbeat payload but keeps Go-native fields for internal coordination.
type Status struct {
	Phase              brevapiv2.BrevCloudNodePhase
	Detail             string
	LastTransitionTime time.Time
}

// Reporter fan-outs status updates to interested consumers and stamps transition times.
// It is safe for concurrent use.
type Reporter struct {
	mu      sync.Mutex
	current Status
	now     func() time.Time

	updates chan Status
}

// Option configures Reporter construction.
type Option func(*Reporter)

// WithClock overrides the wall clock used to stamp LastTransitionTime. Tests may
// inject deterministic values.
func WithClock(now func() time.Time) Option {
	return func(r *Reporter) {
		if now != nil {
			r.now = now
		}
	}
}

// NewReporter constructs a Reporter seeded with an initial status.
func NewReporter(initial Status, opts ...Option) *Reporter {
	r := &Reporter{
		current: initial,
		now:     time.Now,
		updates: make(chan Status, 1),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	if r.current.LastTransitionTime.IsZero() {
		r.current.LastTransitionTime = r.now()
	}
	return r
}

// Updates exposes a read-only channel for consumers who want to react to transitions.
func (r *Reporter) Updates() <-chan Status {
	return r.updates
}

// Current returns the latest status snapshot.
func (r *Reporter) Current() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.current
}

// Publish records the provided status and broadcasts it if it represents a change.
// If LastTransitionTime is zero it gets stamped with the reporter clock.
func (r *Reporter) Publish(next Status) Status {
	r.mu.Lock()
	defer r.mu.Unlock()

	if next.LastTransitionTime.IsZero() {
		next.LastTransitionTime = r.now()
	}

	if statusesEqual(r.current, next) {
		return r.current
	}

	r.current = next
	select {
	case r.updates <- next:
	default:
	}
	return r.current
}

// MarkActive marks the subsystem as healthy.
func (r *Reporter) MarkActive(detail string) Status {
	return r.Publish(Status{
		Phase:  brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ACTIVE,
		Detail: detail,
	})
}

// MarkError marks the subsystem as unhealthy with additional detail.
func (r *Reporter) MarkError(detail string) Status {
	return r.Publish(Status{
		Phase:  brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ERROR,
		Detail: detail,
	})
}

func statusesEqual(a, b Status) bool {
	return a.Phase == b.Phase &&
		a.Detail == b.Detail &&
		a.LastTransitionTime.Equal(b.LastTransitionTime)
}
