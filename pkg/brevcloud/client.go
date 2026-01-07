package brevcloud

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"

	brevapiv2connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/brevapi/v2/brevapiv2connect"
	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
)

// Client wraps brevapiv2 registration/inspection APIs for Spark flows.
//
// It intentionally exposes small, CLI-focused types instead of raw protos.
type Client struct {
	operator brevapiv2connect.BrevCloudOperatorServiceClient
	store    *store.AuthHTTPStore
}

// NewClient constructs a BrevCloud client backed by brevapiv2.BrevCloudOperatorService.
//
// Authentication is delegated to the provided AuthHTTPStore; a fresh access
// token is fetched for each request.
// NewClient constructs a BrevCloud client backed by brevapiv2.BrevCloudOperatorService.
//
// Authentication is delegated to the provided AuthHTTPStore; a fresh access
// token is fetched for each request.
func NewClient(s *store.AuthHTTPStore) *Client {
	if s == nil {
		return &Client{}
	}

	baseURL := config.GlobalConfig.GetDevplaneAPIURL()
	httpClient := &authHTTPClient{
		store: s,
		base:  http.DefaultClient,
	}

	operator := brevapiv2connect.NewBrevCloudOperatorServiceClient(httpClient, baseURL)

	return &Client{
		operator: operator,
		store:    s,
	}
}

// authHTTPClient adapts AuthHTTPStore to connect.HTTPClient by injecting
// a Bearer token on each request. It mimics the OnBeforeRequest behavior
// used by the Resty client in AuthHTTPClient.
type authHTTPClient struct {
	store *store.AuthHTTPStore
	base  *http.Client
}

func (c *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c.store == nil || c.base == nil {
		return nil, breverrors.New("HTTP client not initialized")
	}

	tokens, err := c.store.GetAuthTokens()
	if err == nil && tokens != nil && tokens.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	}

	return c.base.Do(req)
}

// CreateRegistrationIntentRequest is the CLI-friendly payload for minting a
// single-use registration token for a BrevCloud node.
type CreateRegistrationIntentRequest struct {
	CloudCredID string
	OrgID       string
}

// CreateRegistrationIntentResponse is the subset of fields needed by Spark
// enroll flows.
type CreateRegistrationIntentResponse struct {
	BrevCloudNodeID   string
	CloudCredID       string
	RegistrationToken string
	ExpiresAt         string
}

// CreateRegistrationIntent calls brevapiv2.BrevCloudOperatorService.CreateRegistrationIntent.
func (c *Client) CreateRegistrationIntent(ctx context.Context, req CreateRegistrationIntentRequest) (*CreateRegistrationIntentResponse, error) {
	if c == nil || c.operator == nil {
		return nil, breverrors.New("operator client not initialized")
	}

	pbReq := &brevapiv2.CreateRegistrationIntentRequest{
		CloudCredId: req.CloudCredID,
		OrgId:       req.OrgID,
	}

	resp, err := c.operator.CreateRegistrationIntent(ctx, connect.NewRequest(pbReq))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	msg := resp.Msg
	out := &CreateRegistrationIntentResponse{
		BrevCloudNodeID:   msg.GetBrevCloudNodeId(),
		CloudCredID:       msg.GetCloudCredId(),
		RegistrationToken: msg.GetRegistrationToken(),
	}
	if ts := msg.GetExpiresAt(); ts != nil {
		out.ExpiresAt = ts.AsTime().UTC().Format(time.RFC3339)
	}
	return out, nil
}

// BrevCloudNode is a minimal view of a BrevCloud node suitable for status
// polling and user-facing output.
type BrevCloudNode struct {
	ID           string
	CloudCredID  string
	DisplayName  string
	CloudName    string
	FirstSeenAt  string
	LastSeenAt   string
	Phase        string
	AgentVersion string
}

// GetBrevCloudNode fetches node metadata via brevapiv2.BrevCloudOperatorService.GetBrevCloudNode.
func (c *Client) GetBrevCloudNode(ctx context.Context, brevCloudNodeID string) (*BrevCloudNode, error) {
	if c == nil || c.operator == nil {
		return nil, breverrors.New("operator client not initialized")
	}
	if strings.TrimSpace(brevCloudNodeID) == "" {
		return nil, breverrors.New("brev cloud node id is required")
	}

	pbReq := &brevapiv2.GetBrevCloudNodeRequest{
		BrevCloudNodeId: brevCloudNodeID,
	}

	resp, err := c.operator.GetBrevCloudNode(ctx, connect.NewRequest(pbReq))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	msg := resp.Msg
	node := &BrevCloudNode{
		ID:           msg.GetBrevCloudNodeId(),
		CloudCredID:  msg.GetCloudCredId(),
		DisplayName:  msg.GetDisplayName(),
		CloudName:    msg.GetCloudName(),
		AgentVersion: msg.GetAgentVersion(),
		Phase:        mapPhase(msg.GetPhase()),
	}

	if ts := msg.GetFirstSeenAt(); ts != nil {
		node.FirstSeenAt = ts.AsTime().UTC().Format(time.RFC3339)
	}
	if ts := msg.GetLastSeenAt(); ts != nil {
		node.LastSeenAt = ts.AsTime().UTC().Format(time.RFC3339)
	}

	return node, nil
}

// CloudCred represents a BrevCloud credential as seen by Spark flows.
// For now this is only used to attempt "smart" default resolution; when
// unimplemented, callers should fall back gracefully.
type CloudCred struct {
	ID         string
	ProviderID string
	Labels     map[string]string
}

// ListCloudCred is a placeholder for future integration with a BrevCloud
// cloud credential listing API. Today it returns an error so callers can
// fall back to explicit --cloud-cred-id flags.
func (c *Client) ListCloudCred(ctx context.Context) ([]CloudCred, error) {
	_ = ctx
	return nil, fmt.Errorf("ListCloudCred not implemented")
}

// MockRegistrationIntent constructs a safe, in-memory registration intent
// for demos or tests when opts.mockRegistration is true.
func MockRegistrationIntent(cloudCredID string) CreateRegistrationIntentResponse {
	return CreateRegistrationIntentResponse{
		BrevCloudNodeID:   "brev-mock-node",
		CloudCredID:       cloudCredID,
		RegistrationToken: "brev-mock-registration-token",
		ExpiresAt:         time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339),
	}
}

func mapPhase(status *brevapiv2.BrevCloudNodeStatus) string {
	if status == nil {
		return ""
	}
	switch status.GetPhase() {
	case brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_WAITING_FOR_REGISTRATION:
		return "WAITING_FOR_REGISTRATION"
	case brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ACTIVE:
		return "ACTIVE"
	case brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_OFFLINE:
		return "OFFLINE"
	case brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_STOPPED:
		return "STOPPED"
	case brevapiv2.BrevCloudNodePhase_BREV_CLOUD_NODE_PHASE_ERROR:
		return "ERROR"
	default:
		return ""
	}
}
