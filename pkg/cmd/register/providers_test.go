package register

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type netBirdCall struct {
	name string
	args []string
}

type netBirdResult struct {
	output []byte
	err    error
}

type fakeNetBirdCommandRunner struct {
	results  []netBirdResult
	fallback netBirdResult
	calls    []netBirdCall
}

func (f *fakeNetBirdCommandRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, netBirdCall{name: name, args: append([]string(nil), args...)})
	if len(f.results) == 0 {
		return append([]byte(nil), f.fallback.output...), f.fallback.err
	}
	result := f.results[0]
	f.results = f.results[1:]
	return append([]byte(nil), result.output...), result.err
}

func (f *fakeNetBirdCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	_, err := f.Output(ctx, name, args...)
	return err
}

func connectedNetBirdStatus() []byte {
	return []byte("Management: Connected\n")
}

func disconnectedNetBirdStatus() []byte {
	return []byte("Management: Disconnected\n")
}

func newTestNetbird(runner *fakeNetBirdCommandRunner) Netbird {
	return Netbird{
		runner:         runner,
		connectTimeout: 10 * time.Millisecond,
		pollInterval:   time.Millisecond,
	}
}

func TestNetbirdEnsureConnected_AlreadyConnectedDoesNotReconnect(t *testing.T) {
	runner := &fakeNetBirdCommandRunner{results: []netBirdResult{
		{output: []byte("active\n")},
		{output: connectedNetBirdStatus()},
	}}

	err := newTestNetbird(runner).EnsureConnected(context.Background())
	if err != nil {
		t.Fatalf("EnsureConnected() error = %v", err)
	}

	wantCalls := []netBirdCall{
		{name: "systemctl", args: []string{"is-active", "netbird"}},
		{name: "netbird", args: []string{"status"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestNetbirdEnsureConnected_StartsInactiveService(t *testing.T) {
	runner := &fakeNetBirdCommandRunner{results: []netBirdResult{
		{output: []byte("inactive\n")},
		{},
		{output: connectedNetBirdStatus()},
	}}

	err := newTestNetbird(runner).EnsureConnected(context.Background())
	if err != nil {
		t.Fatalf("EnsureConnected() error = %v", err)
	}

	wantCalls := []netBirdCall{
		{name: "systemctl", args: []string{"is-active", "netbird"}},
		{name: "sudo", args: []string{"systemctl", "start", "netbird"}},
		{name: "netbird", args: []string{"status"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestNetbirdEnsureConnected_ReconnectsAndWaitsForConfirmation(t *testing.T) {
	runner := &fakeNetBirdCommandRunner{results: []netBirdResult{
		{output: []byte("active\n")},
		{output: disconnectedNetBirdStatus()},
		{},
		{output: disconnectedNetBirdStatus()},
		{output: connectedNetBirdStatus()},
	}}

	err := newTestNetbird(runner).EnsureConnected(context.Background())
	if err != nil {
		t.Fatalf("EnsureConnected() error = %v", err)
	}

	wantCalls := []netBirdCall{
		{name: "systemctl", args: []string{"is-active", "netbird"}},
		{name: "netbird", args: []string{"status"}},
		{name: "sudo", args: []string{"netbird", "up"}},
		{name: "netbird", args: []string{"status"}},
		{name: "netbird", args: []string{"status"}},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("commands = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestNetbirdEnsureConnected_ReconnectFailure(t *testing.T) {
	runner := &fakeNetBirdCommandRunner{results: []netBirdResult{
		{output: []byte("active\n")},
		{output: disconnectedNetBirdStatus()},
		{err: errors.New("up failed")},
	}}

	err := newTestNetbird(runner).EnsureConnected(context.Background())
	if err == nil || !strings.Contains(err.Error(), "failed to reconnect Brev tunnel") {
		t.Fatalf("EnsureConnected() error = %v, want reconnect failure context", err)
	}
}

func TestNetbirdEnsureConnected_StatusNeverConfirmsConnection(t *testing.T) {
	runner := &fakeNetBirdCommandRunner{
		results: []netBirdResult{
			{output: []byte("active\n")},
			{output: disconnectedNetBirdStatus()},
			{},
		},
		fallback: netBirdResult{output: disconnectedNetBirdStatus()},
	}

	err := newTestNetbird(runner).EnsureConnected(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Brev tunnel connection was not confirmed") {
		t.Fatalf("EnsureConnected() error = %v, want confirmation timeout", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("EnsureConnected() error = %v, want context deadline exceeded", err)
	}
}

func TestNetbirdEnsureConnected_StatusErrorsAreNotSuccess(t *testing.T) {
	statusErr := errors.New("netbird status unavailable")
	runner := &fakeNetBirdCommandRunner{
		results: []netBirdResult{
			{output: []byte("active\n")},
			{err: statusErr},
			{},
		},
		fallback: netBirdResult{err: statusErr},
	}

	err := newTestNetbird(runner).EnsureConnected(context.Background())
	if err == nil {
		t.Fatal("EnsureConnected() error = nil, want confirmation timeout")
	}
	if !strings.Contains(err.Error(), "Brev tunnel connection was not confirmed") || !strings.Contains(err.Error(), statusErr.Error()) {
		t.Fatalf("EnsureConnected() error = %v, want timeout containing latest status failure", err)
	}
}
