package store

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
)

type failingAuth struct{}

func (failingAuth) GetAccessToken() (string, error) {
	return "", errors.New("token unavailable")
}

func TestAuthenticatedHTTPClientInjectsBearerToken(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAuthenticatedHTTPClient(MockAuth{})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // test

	assert.Equal(t, "Bearer mock-token", gotAuth)
}

func TestAuthenticatedHTTPClientAddsCLIAttributionParams(t *testing.T) {
	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAuthenticatedHTTPClient(MockAuth{})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		server.URL+"/devplaneapi.v1.ExternalNodeService/ListNodes?connect=v1", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // test

	// Same attribution the REST client sends, so dev-plane sees one CLI identity.
	assert.Equal(t, "cli", gotQuery.Get("utm_source"))
	assert.Equal(t, version.Version, gotQuery.Get("cli_version"))
	assert.Equal(t, runtime.GOOS, gotQuery.Get("os"))
	// ConnectRPC's own protocol params must survive.
	assert.Equal(t, "v1", gotQuery.Get("connect"))
}

func TestAuthenticatedHTTPClientPropagatesTokenError(t *testing.T) {
	client := NewAuthenticatedHTTPClient(failingAuth{})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close() //nolint:errcheck // test
		t.Fatal("expected error from token provider")
	}
}
