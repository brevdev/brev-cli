package store

import (
	"bytes"
	"errors"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	resty "github.com/go-resty/resty/v2"
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

// newErrResponse spins up a one-shot server returning the given status + body and
// returns the resty response, so tests exercise the real HTTPResponseError path.
// authToken, if set, is applied via SetAuthToken (populating Request.Token, which
// is how requestUsedBrevAPIKey detects a bak- API key).
func newErrResponse(t *testing.T, status int, body, authToken string) *resty.Response {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	req := resty.New().SetBaseURL(srv.URL).R()
	if authToken != "" {
		req.SetAuthToken(authToken)
	}
	resp, err := req.Get("/api/organizations")
	if err != nil {
		t.Fatalf("transport error (not under test): %v", err)
	}
	return resp
}

// Control-plane 401 via interactive login: typed-but-messageless body, non-API-key
// token -> friendly "brev login" message.
func TestHTTPResponseError_ControlPlane401_Interactive(t *testing.T) {
	resp := newErrResponse(t, http.StatusUnauthorized, `{"errors":[{"type":"UnauthorizedError"}]}`, "header.payload.sig")
	got := NewHTTPResponseError(resp).Error()

	if !strings.Contains(got, "brev login") {
		t.Fatalf("want a `brev login` hint, got: %q", got)
	}
	if strings.Contains(got, "API key") {
		t.Fatalf("interactive-login failure must not mention API key, got: %q", got)
	}
}

// Same control-plane 401 but authed with a bak- API key -> API-key message,
// not "logged out / brev login".
func TestHTTPResponseError_ControlPlane401_APIKey(t *testing.T) {
	resp := newErrResponse(t, http.StatusUnauthorized, `{"errors":[{"type":"UnauthorizedError"}]}`, "bak-not-a-real-key")
	got := NewHTTPResponseError(resp).Error()

	if !strings.Contains(got, "API key") {
		t.Fatalf("want an API-key message, got: %q", got)
	}
	if strings.Contains(got, "logged out") {
		t.Fatalf("API-key failure must not say 'logged out', got: %q", got)
	}
}

// GitHub's bad-credentials 401 (top-level message, no errors[].type) must NOT be
// mapped to a brev-login message; it should surface GitHub's own error (the
// `brev upgrade` false positive).
func TestHTTPResponseError_GitHub401_NotBrevLogin(t *testing.T) {
	resp := newErrResponse(t, http.StatusUnauthorized, `{"message":"Bad credentials","status":"401"}`, "")
	got := NewHTTPResponseError(resp).Error()

	if strings.Contains(got, "brev login") {
		t.Fatalf("GitHub 401 must not be mapped to `brev login`, got: %q", got)
	}
	if !strings.Contains(got, "Bad credentials") {
		t.Fatalf("GitHub 401 should surface its own message, got: %q", got)
	}
}

// A real server message must still pass through unchanged (regression guard).
func TestHTTPResponseError_ServerMessage_PassesThrough(t *testing.T) {
	resp := newErrResponse(t, http.StatusBadRequest, `{"errors":[{"type":"BadRequestError","message":"instance type invalid"}]}`, "")
	got := NewHTTPResponseError(resp).Error()

	if !strings.Contains(got, "instance type invalid") {
		t.Fatalf("server-provided message should pass through, got: %q", got)
	}
}
