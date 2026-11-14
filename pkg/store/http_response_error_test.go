package store

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	resty "github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const unauthorizedBody = `{"errors":[{"type":"UnauthorizedError"}]}`

func doRequestWithToken(t *testing.T, status int, body, token string) *HTTPResponseError {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	req := resty.New().R()
	if token != "" {
		req.SetAuthToken(token)
	}
	res, err := req.Get(server.URL)
	require.NoError(t, err)
	require.True(t, res.IsError())
	return NewHTTPResponseError(res)
}

func TestHTTPResponseError_UnauthorizedWithAPIKeyShowsHintThenOriginalMessage(t *testing.T) {
	err := doRequestWithToken(t, http.StatusUnauthorized, unauthorizedBody, "bak-expired")

	msg := err.Error()
	assert.True(t, strings.HasPrefix(msg, expiredAPIKeyMessage+"\n"), "hint should come first: %q", msg)
	assert.Contains(t, msg, "brev login --api-key")
	// The base error is preserved after the hint. baseError() renders the
	// typed-but-messageless control-plane 401 as the friendly key-expired copy
	// (see the UnauthorizedError branch), not the raw "401 Unauthorized" line.
	assert.True(t, strings.HasSuffix(msg, err.baseError()))
	assert.Contains(t, msg, "your Brev API key is invalid or expired")
}

func TestHTTPResponseError_UnauthorizedWithoutAPIKeyKeepsOriginalMessage(t *testing.T) {
	// A non-API-key (e.g. browser login JWT) 401 must not claim the API key is bad.
	err := doRequestWithToken(t, http.StatusUnauthorized, unauthorizedBody, "eyJhbGciOiJIUzI1NiJ9.e30.sig")

	msg := err.Error()
	assert.NotContains(t, msg, expiredAPIKeyMessage)
	// A messageless typed 401 renders the friendly logged-out copy; the raw
	// status line only appears for bodies the control-plane mapping can't interpret.
	assert.Contains(t, msg, "you're logged out")
}

func TestHTTPResponseError_UnauthorizedWithNoTokenKeepsOriginalMessage(t *testing.T) {
	err := doRequestWithToken(t, http.StatusUnauthorized, unauthorizedBody, "")

	assert.NotContains(t, err.Error(), expiredAPIKeyMessage)
}

func TestHTTPResponseError_NonUnauthorizedWithAPIKeyKeepsOriginalMessage(t *testing.T) {
	// Only a 401 means the key is expired; other failures must surface as-is.
	err := doRequestWithToken(t, http.StatusForbidden, `{"errors":[{"message":"forbidden"}]}`, "bak-valid")

	assert.Equal(t, "forbidden\n", err.Error())
}
