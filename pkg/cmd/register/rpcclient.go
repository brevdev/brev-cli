package register

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// DevPlaneBaseURL is the base URL for the dev-plane API.
// TODO: source from config once the URL is finalized.
const DevPlaneBaseURL = "https://brevapi.us-west-2-prod.control-plane.brev.dev"

// TODO: Replace these local types with generated proto types once the
// ExternalNodeService is published to buf.build:
//
//   import (
//       nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
//       nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
//   )

// AddNodeRequest matches the proto AddNodeRequest message.
type AddNodeRequest struct {
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	DeviceID       string    `json:"device_id"`
	NodeSpec       *NodeSpec `json:"node_spec"`
}

// AddNodeResponse matches the proto AddNodeResponse message.
type AddNodeResponse struct {
	ExternalNode  *ExternalNode     `json:"external_node"`
	SetupCommands map[string]string `json:"setup_commands"`
}

// ExternalNode matches the proto ExternalNode message (subset of fields we need).
type ExternalNode struct {
	ExternalNodeID string `json:"external_node_id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	DeviceID       string `json:"device_id"`
}

// RemoveNodeRequest matches the proto RemoveNodeRequest message.
type RemoveNodeRequest struct {
	ExternalNodeID string `json:"external_node_id"`
	OrganizationID string `json:"organization_id"`
}

// ExternalNodeServiceClient defines the RPCs we call from the CLI.
// This will be replaced by the generated ConnectRPC client interface
// once the service is published.
type ExternalNodeServiceClient interface {
	AddNode(ctx context.Context, req *AddNodeRequest) (*AddNodeResponse, error)
	RemoveNode(ctx context.Context, req *RemoveNodeRequest) error
}

// tokenProvider abstracts access token retrieval for the HTTP transport.
type tokenProvider interface {
	GetAccessToken() (string, error)
}

// bearerTokenTransport injects a Bearer token into every request.
type bearerTokenTransport struct {
	provider tokenProvider
	base     http.RoundTripper
}

func (t *bearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.provider.GetAccessToken()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(req)
}

// newAuthenticatedHTTPClient creates an http.Client that injects the bearer token
// from the given provider on every request.
func newAuthenticatedHTTPClient(provider tokenProvider) *http.Client {
	return &http.Client{
		Transport: &bearerTokenTransport{
			provider: provider,
			base:     http.DefaultTransport,
		},
	}
}

// ConnectNodeClient is a temporary REST-based implementation of ExternalNodeServiceClient.
// It will be replaced by the generated ConnectRPC client once the service proto
// is published to buf.build.
//
// TODO: Replace with:
//
//	httpClient := newAuthenticatedHTTPClient(store)
//	client := nodev1connect.NewExternalNodeServiceClient(httpClient, baseURL)
type ConnectNodeClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewConnectNodeClient creates a new ConnectNodeClient.
func NewConnectNodeClient(provider tokenProvider, baseURL string) *ConnectNodeClient {
	return &ConnectNodeClient{
		httpClient: newAuthenticatedHTTPClient(provider),
		baseURL:    baseURL,
	}
}

func (c *ConnectNodeClient) AddNode(ctx context.Context, req *AddNodeRequest) (*AddNodeResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal AddNodeRequest: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/devplaneapi.v1.ExternalNodeService/AddNode", bytes.NewReader(body))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("AddNode request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AddNode returned status %d", resp.StatusCode)
	}

	var result AddNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode AddNode response: %w", err)
	}
	return &result, nil
}

func (c *ConnectNodeClient) RemoveNode(ctx context.Context, req *RemoveNodeRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal RemoveNodeRequest: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/devplaneapi.v1.ExternalNodeService/RemoveNode", bytes.NewReader(body))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("RemoveNode request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("RemoveNode returned status %d", resp.StatusCode)
	}
	return nil
}
