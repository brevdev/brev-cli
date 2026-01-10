package health

import (
	"testing"
	"time"

	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
)

func TestNewReporterSeedsTimestamp(t *testing.T) {
	base := time.Unix(100, 0)
	reporter := NewReporter(Status{}, WithClock(func() time.Time {
		return base
	}))

	got := reporter.Current()
	if !got.LastTransitionTime.Equal(base) {
		t.Fatalf("LastTransitionTime = %s, want %s", got.LastTransitionTime, base)
	}
}

func TestReporterTransitionsEmitUpdates(t *testing.T) {
	clockValues := []time.Time{
		time.Unix(10, 0), // initial
		time.Unix(20, 0), // error transition
		time.Unix(30, 0), // active transition
	}
	var clockIdx int
	reporter := NewReporter(Status{}, WithClock(func() time.Time {
		v := clockValues[clockIdx]
		if clockIdx < len(clockValues)-1 {
			clockIdx++
		}
		return v
	}))

	updates := reporter.Updates()

	reporter.MarkError("missing binary")
	select {
	case got := <-updates:
		if got.Phase != brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ERROR {
			t.Fatalf("Phase = %v, want BrevCloudNodePhase_ERROR", got.Phase)
		}
		if got.Detail != "missing binary" {
			t.Fatalf("Detail = %q, want %q", got.Detail, "missing binary")
		}
		if !got.LastTransitionTime.Equal(clockValues[1]) {
			t.Fatalf("LastTransitionTime = %s, want %s", got.LastTransitionTime, clockValues[1])
		}
	case <-time.After(time.Second):
		t.Fatalf("expected update for error transition")
	}

	reporter.MarkActive("recovered")
	select {
	case got := <-updates:
		if got.Phase != brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ACTIVE {
			t.Fatalf("Phase = %v, want BrevCloudNodePhase_ACTIVE", got.Phase)
		}
		if got.Detail != "recovered" {
			t.Fatalf("Detail = %q, want %q", got.Detail, "recovered")
		}
		if !got.LastTransitionTime.Equal(clockValues[2]) {
			t.Fatalf("LastTransitionTime = %s, want %s", got.LastTransitionTime, clockValues[2])
		}
	case <-time.After(time.Second):
		t.Fatalf("expected update for recovery transition")
	}
}

func TestReporterPublishNoChangeDoesNotEmit(t *testing.T) {
	initial := Status{
		Phase:              brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ACTIVE,
		Detail:             "ok",
		LastTransitionTime: time.Unix(42, 0),
	}
	reporter := NewReporter(initial)
	updates := reporter.Updates()

	reporter.Publish(initial)

	select {
	case <-updates:
		t.Fatalf("expected no update for identical publish")
	default:
	}

	current := reporter.Current()
	if current != initial {
		t.Fatalf("Current changed = %+v, want %+v", current, initial)
	}
}
