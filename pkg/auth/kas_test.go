package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKasDoDeviceAuthFlow_CallsV2LoginThenPollsPing(t *testing.T) {
	const (
		userCode = "ABCD-1234"
		session  = "session-key-abc"
		verify   = "https://brev.nvidia.com/cli-login?code=xyz"
	)

	var loginCalls, pingCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v2/device/login":
			atomic.AddInt32(&loginCalls, 1)
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "user@example.com", body["email"])
			assert.Equal(t, "https://brev.nvidia.com/cli-login", body["redirectUri"])
			assert.NotEmpty(t, body["deviceId"])

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"user_code":        userCode,
				"session_key":      session,
				"verification_url": verify,
			})
		case r.Method == http.MethodHead && r.URL.Path == "/v2/ping":
			call := atomic.AddInt32(&pingCalls, 1)
			assert.Equal(t, "Bearer "+session, r.Header.Get("Authorization"))
			if call < 2 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := NewKasAuthenticator("user@example.com", srv.URL, "https://login.nvidia.com", false, "https://brev.nvidia.com/cli-login")

	var gotURL, gotCode string
	tokens, err := a.DoDeviceAuthFlow(func(url, code string) {
		gotURL, gotCode = url, code
	})
	require.NoError(t, err)

	assert.Equal(t, verify, gotURL)
	assert.Equal(t, userCode, gotCode)
	assert.Equal(t, session, tokens.AccessToken, "session_key must be usable as the bearer token")
	assert.Empty(t, tokens.RefreshToken, "v2 login does not issue a refresh token")
	assert.Equal(t, int32(1), atomic.LoadInt32(&loginCalls))
	assert.GreaterOrEqual(t, atomic.LoadInt32(&pingCalls), int32(2), "must keep polling until ping succeeds")
}

func TestKasDoDeviceAuthFlow_TimesOutWhenPingNeverSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v2/device/login" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"user_code":"c","session_key":"s","verification_url":"u"}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := NewKasAuthenticator("user@example.com", srv.URL, "https://login.nvidia.com", false, "")
	a.PollTimeout = 50 * time.Millisecond

	_, err := a.DoDeviceAuthFlow(func(_, _ string) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestKasDoDeviceAuthFlow_LoginErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer srv.Close()

	a := NewKasAuthenticator("user@example.com", srv.URL, "https://login.nvidia.com", false, "")

	flowCalled := false
	_, err := a.DoDeviceAuthFlow(func(_, _ string) { flowCalled = true })
	require.Error(t, err)
	assert.False(t, flowCalled, "login flow must not start when the device login call fails")
}
