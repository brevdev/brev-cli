package register

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockTokenProvider struct {
	token string
	err   error
}

func (m *mockTokenProvider) GetAccessToken() (string, error) {
	return m.token, m.err
}

func Test_bearerTokenTransport_InjectsHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &mockTokenProvider{token: "test-token-123"}
	client := newAuthenticatedHTTPClient(provider)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // test

	if gotAuth != "Bearer test-token-123" {
		t.Errorf("expected 'Bearer test-token-123', got %q", gotAuth)
	}
}

func Test_bearerTokenTransport_PropagatesTokenError(t *testing.T) {
	provider := &mockTokenProvider{err: http.ErrAbortHandler}
	client := newAuthenticatedHTTPClient(provider)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close() //nolint:errcheck // test
		t.Fatal("expected error from token provider")
	}
}

func Test_ConnectNodeClient_AddNode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/devplaneapi.v1.ExternalNodeService/AddNode" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}

		var req AddNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.OrganizationID != "org_123" {
			t.Errorf("unexpected org ID: %s", req.OrganizationID)
		}
		if req.Name != "My Spark" {
			t.Errorf("unexpected name: %s", req.Name)
		}

		resp := AddNodeResponse{
			ExternalNode: &ExternalNode{
				ExternalNodeID: "unode_abc",
				OrganizationID: "org_123",
				Name:           "My Spark",
				DeviceID:       req.DeviceID,
			},
			SetupCommands: map[string]string{"netbird": "netbird up"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck // test
	}))
	defer server.Close()

	provider := &mockTokenProvider{token: "tok"}
	client := NewConnectNodeClient(provider, server.URL)

	resp, err := client.AddNode(context.Background(), &AddNodeRequest{
		OrganizationID: "org_123",
		Name:           "My Spark",
		DeviceID:       "dev-uuid",
		NodeSpec:       &NodeSpec{Architecture: "arm64"},
	})
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}
	if resp.ExternalNode.ExternalNodeID != "unode_abc" {
		t.Errorf("unexpected node ID: %s", resp.ExternalNode.ExternalNodeID)
	}
	if len(resp.SetupCommands) != 1 {
		t.Errorf("expected 1 setup command, got %d", len(resp.SetupCommands))
	}
}

func Test_ConnectNodeClient_AddNode_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := &mockTokenProvider{token: "tok"}
	client := NewConnectNodeClient(provider, server.URL)

	_, err := client.AddNode(context.Background(), &AddNodeRequest{
		OrganizationID: "org_123",
		Name:           "Test",
		DeviceID:       "dev",
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func Test_ConnectNodeClient_RemoveNode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/devplaneapi.v1.ExternalNodeService/RemoveNode" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req RemoveNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.ExternalNodeID != "unode_abc" {
			t.Errorf("unexpected node ID: %s", req.ExternalNodeID)
		}
		if req.OrganizationID != "org_123" {
			t.Errorf("unexpected org ID: %s", req.OrganizationID)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &mockTokenProvider{token: "tok"}
	client := NewConnectNodeClient(provider, server.URL)

	err := client.RemoveNode(context.Background(), &RemoveNodeRequest{
		ExternalNodeID: "unode_abc",
		OrganizationID: "org_123",
	})
	if err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}
}

func Test_ConnectNodeClient_RemoveNode_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := &mockTokenProvider{token: "tok"}
	client := NewConnectNodeClient(provider, server.URL)

	err := client.RemoveNode(context.Background(), &RemoveNodeRequest{
		ExternalNodeID: "unode_missing",
		OrganizationID: "org_123",
	})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}
