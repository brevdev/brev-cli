package register

import (
	"net/http"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// TokenProvider abstracts access token retrieval for the HTTP transport.
type TokenProvider interface {
	GetAccessToken() (string, error)
}

// bearerTokenTransport injects a Bearer token into every request.
type bearerTokenTransport struct {
	provider TokenProvider
	base     http.RoundTripper
}

func (t *bearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.provider.GetAccessToken()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return resp, nil
}

// newAuthenticatedHTTPClient creates an http.Client that injects the bearer token
// from the given provider on every request.
func newAuthenticatedHTTPClient(provider TokenProvider) *http.Client {
	return &http.Client{
		Transport: &bearerTokenTransport{
			provider: provider,
			base:     http.DefaultTransport,
		},
	}
}

// NewNodeServiceClient creates a ConnectRPC ExternalNodeServiceClient using the
// given token provider for authentication.
func NewNodeServiceClient(provider TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient {
	return nodev1connect.NewExternalNodeServiceClient(
		newAuthenticatedHTTPClient(provider),
		baseURL,
	)
}

// toProtoNodeSpec converts the local NodeSpec (used for collection, display, persistence)
// to the generated proto NodeSpec for RPC calls.
func toProtoNodeSpec(s *NodeSpec) *nodev1.NodeSpec {
	if s == nil {
		return nil
	}

	proto := &nodev1.NodeSpec{
		RamBytes: s.RAMBytes,
		CpuCount: s.CPUCount,
	}

	for _, st := range s.Storage {
		proto.Storage = append(proto.Storage, &nodev1.StorageSpec{
			StorageBytes: st.StorageBytes,
			StorageType:  st.StorageType,
		})
	}

	if s.Architecture != "" {
		proto.Architecture = &s.Architecture
	}
	if s.OS != "" {
		proto.Os = &s.OS
	}
	if s.OSVersion != "" {
		proto.OsVersion = &s.OSVersion
	}

	for _, g := range s.GPUs {
		pg := &nodev1.GPUSpec{
			Model:       g.Model,
			Count:       g.Count,
			MemoryBytes: g.MemoryBytes,
		}
		proto.Gpus = append(proto.Gpus, pg)
	}

	return proto
}
