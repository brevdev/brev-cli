package auth

import (
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func Test_hostFromURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://api.ngc.nvidia.com", "api.ngc.nvidia.com"},
		{"https://api.ngc.nvidia.com/token", "api.ngc.nvidia.com"},
		{"http://localhost:8080", "localhost:8080"},
		{"", ""},
		{"://nonsense", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, hostFromURL(tc.in))
		})
	}
}

// closedServerURL returns a URL that is guaranteed to fail to connect: an
// httptest.Server that has already been Close()d.
func closedServerURL(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	return server.URL
}

// MakeLoginCall against an unreachable host should surface a friendly
// *NetworkError instead of the raw transport error, and the host should
// match the BaseURL we tried to reach.
func TestMakeLoginCall_NetworkError(t *testing.T) {
	baseURL := closedServerURL(t)
	auth := KasAuthenticator{BaseURL: baseURL}

	_, err := auth.MakeLoginCall("device-id", "user@example.com")
	if !assert.Error(t, err) {
		return
	}

	var netErr *breverrors.NetworkError
	if !assert.True(t, stderrors.As(err, &netErr), "expected *NetworkError, got %T: %v", err, err) {
		return
	}

	expectedHost, _ := url.Parse(baseURL)
	assert.Equal(t, expectedHost.Host, netErr.Host)
	assert.Contains(t, netErr.Error(), "internet connection")
}

// retrieveIDToken against an unreachable host should surface a friendly
// *NetworkError. This is the path hit by `brev shell` when the cached
// access token has expired and api.ngc.nvidia.com is unreachable.
func TestRetrieveIDToken_NetworkError(t *testing.T) {
	baseURL := closedServerURL(t)
	auth := KasAuthenticator{BaseURL: baseURL}

	_, err := auth.retrieveIDToken("session-key", "device-id")
	if !assert.Error(t, err) {
		return
	}

	var netErr *breverrors.NetworkError
	if !assert.True(t, stderrors.As(err, &netErr), "expected *NetworkError, got %T: %v", err, err) {
		return
	}

	expectedHost, _ := url.Parse(baseURL)
	assert.Equal(t, expectedHost.Host, netErr.Host)
}

// retrieveIDToken against a server that returns HTTP 4xx/5xx (i.e. a
// non-network error) should NOT be classified as a network error.
func TestRetrieveIDToken_NonNetworkErrorUnchanged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"requestStatus":{"statusCode":"INTERNAL_ERROR"}}`))
	}))
	defer server.Close()

	auth := KasAuthenticator{BaseURL: server.URL}
	_, err := auth.retrieveIDToken("session-key", "device-id")
	if !assert.Error(t, err) {
		return
	}

	var netErr *breverrors.NetworkError
	assert.False(t, stderrors.As(err, &netErr), "HTTP error should not be classified as a network error: %v", err)
	assert.Contains(t, err.Error(), "status code: 500")
}
