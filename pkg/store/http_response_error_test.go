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
	// the original error is preserved after the hint
	assert.True(t, strings.HasSuffix(msg, err.baseError()))
	assert.Contains(t, msg, "401 Unauthorized")
	assert.Contains(t, msg, "UnauthorizedError")
}

func TestHTTPResponseError_UnauthorizedWithoutAPIKeyKeepsOriginalMessage(t *testing.T) {
	// A non-API-key (e.g. browser login JWT) 401 must not claim the API key is bad.
	err := doRequestWithToken(t, http.StatusUnauthorized, unauthorizedBody, "eyJhbGciOiJIUzI1NiJ9.e30.sig")

	msg := err.Error()
	assert.NotContains(t, msg, expiredAPIKeyMessage)
	assert.Contains(t, msg, "401")
	assert.Contains(t, msg, "UnauthorizedError")
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
