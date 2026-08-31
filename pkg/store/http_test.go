package store

import (
	"bytes"
	"errors"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockAuth struct{ token *string }

func (a MockAuth) GetAccessToken() (string, error) {
	if a.token == nil {
		return "mock-token", nil
	}
	return *a.token, nil
}

func MakeMockNoHTTPStore() *NoAuthHTTPStore {
	fs := MakeMockFileStore()
	nh := fs.WithNoAuthHTTPClient(NewNoAuthHTTPClient(""))
	return nh
}

func MakeMockAuthHTTPStore() *AuthHTTPStore {
	nh := MakeMockNoHTTPStore()
	ah := nh.WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{}, ""))
	return ah
}

func TestQuietRestyLogger_SuppressesDeclineLogin(t *testing.T) {
	var buf bytes.Buffer
	base := &testLogger{out: &buf}
	q := quietRestyLogger{next: base}

	q.Errorf("%v", errors.New(breverrors.DeclineToLoginMessage))
	q.Warnf("%v, Attempt %v", errors.New(breverrors.DeclineToLoginMessage), 1)
	q.Errorf("%v", errors.New("connection refused"))
	q.Warnf("some other warning")

	out := buf.String()
	assert.NotContains(t, out, "declined to login")
	assert.Contains(t, out, "connection refused")
	assert.Contains(t, out, "some other warning")
}

func TestQuietRestyLogger_DebugPassesThrough(t *testing.T) {
	var buf bytes.Buffer
	base := &testLogger{out: &buf}
	q := quietRestyLogger{next: base}

	q.Debugf("debug %s", "detail")
	assert.Contains(t, buf.String(), "debug detail")
}

func TestIsDeclinedLoginMsg(t *testing.T) {
	assert.True(t, isDeclinedLoginMsg("%v", errors.New("declined to login")))
	assert.False(t, isDeclinedLoginMsg("%v", errors.New("boom")))
	// Non-%v formats carry no embedded error; never filtered.
	assert.False(t, isDeclinedLoginMsg("plain format"))
}

type testLogger struct {
	out *bytes.Buffer
}

func (t *testLogger) Errorf(format string, v ...interface{}) { t.writef(format, v...) }
func (t *testLogger) Warnf(format string, v ...interface{})  { t.writef(format, v...) }
func (t *testLogger) Debugf(format string, v ...interface{}) { t.writef(format, v...) }

func (t *testLogger) writef(format string, v ...interface{}) {
	fmt.Fprintf(t.out, format, v...)
}

// declineAuth simulates a user answering "n" at the login prompt.
type declineAuth struct{}

func (declineAuth) GetAccessToken() (string, error) {
	return "", &breverrors.DeclineToLoginError{}
}

// The exact scenario from the bug report: a command runs, the user declines
// login, and the request fails. Resty must NOT spray WARN/ERROR retry chatter
// with stack-traced wrappers to stderr; the error must surface as a clean
// sentinel for DisplayAndHandleError to render.
func TestNewAuthHTTPClient_DeclinedLoginIsQuietAndClean(t *testing.T) {
	// NO sink replacement: the factory-installed logger chain must handle
	// this itself. Capture stderr (where the factory's logger writes) and
	// assert the decline chatter never reaches it.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = r.Close()
		_ = w.Close()
	})

	client := NewAuthHTTPClient(declineAuth{}, "https://api.test") // installs quietRestyLogger{next: stderrLogger}
	client.restyClient.SetRetryCount(1)
	client.restyClient.SetTimeout(2 * time.Second)

	_, err = client.restyClient.R().Get("/user")

	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stderr = origStderr

	require.Error(t, err)
	var decline *breverrors.DeclineToLoginError
	require.True(t, breverrors.As(err, &decline), "decline sentinel must survive the resty round trip (wrapping allowed)")
	require.True(t, stderrors.Is(err, decline), "DisplayAndHandleError matches the sentinel with errors.Is through wrapping")
	assert.NotContains(t, buf.String(), "declined to login", "factory logger must suppress decline retry chatter")
}

// The factory must install quietRestyLogger over a REAL sink: unrelated
// errors still reach stderr (only declined-login chatter is filtered).
func TestNewAuthHTTPClient_LoggerForwardsUnrelatedErrors(t *testing.T) {
	// Replace stderr before construction so the factory's stderrLogger
	// captures our pipe.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = r.Close()
		_ = w.Close()
	})

	client := NewAuthHTTPClient(errorAuth{}, "https://api.test")
	client.restyClient.SetRetryCount(1)
	client.restyClient.SetTimeout(2 * time.Second)

	_, err = client.restyClient.R().Get("/user")

	// Flush the pipe before restoring.
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stderr = origStderr

	require.Error(t, err)
	assert.Contains(t, buf.String(), "ERROR RESTY", "unrelated auth errors must still be logged by the factory logger")
	assert.Contains(t, buf.String(), "boom-auth", "the actual error text must reach the sink")
}

// errorAuth fails auth with a non-decline error: must be loud.
type errorAuth struct{}

func (errorAuth) GetAccessToken() (string, error) {
	return "", errors.New("boom-auth")
}
