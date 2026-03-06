package register

import (
	"net/http"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/externalnode"
)

// bearerTokenTransport injects a Bearer token into every request.
// We use a custom RoundTripper instead of setting headers on individual
// client.Do() calls because ConnectRPC owns the HTTP requests internally —
// this is the only hook we have to add auth headers.
type bearerTokenTransport struct {
	provider externalnode.TokenProvider
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
func newAuthenticatedHTTPClient(provider externalnode.TokenProvider) *http.Client {
	return &http.Client{
		Transport: &bearerTokenTransport{
			provider: provider,
			base:     http.DefaultTransport,
		},
	}
}

// NewNodeServiceClient creates a ConnectRPC ExternalNodeServiceClient using the
// given token provider for authentication.
func NewNodeServiceClient(provider externalnode.TokenProvider, baseURL string) nodev1connect.ExternalNodeServiceClient {
	return nodev1connect.NewExternalNodeServiceClient(
		newAuthenticatedHTTPClient(provider),
		baseURL,
	)
}

// toProtoNodeSpec converts the local HardwareProfile (used for collection, display,
// persistence) to the generated proto NodeSpec for RPC calls.
func toProtoNodeSpec(hw *HardwareProfile) *nodev1.NodeSpec {
	if hw == nil {
		return nil
	}

	proto := &nodev1.NodeSpec{
		RamBytes: hw.RAMBytes,
		CpuCount: hw.CPUCount,
	}

	for _, st := range hw.Storage {
		storageSpec := &nodev1.StorageSpec{
			StorageBytes: st.StorageBytes,
			StorageType:  st.StorageType,
		}
		if st.Name != "" {
			storageSpec.Device = &st.Name
		}
		proto.Storage = append(proto.Storage, storageSpec)
	}

	if hw.Architecture != "" {
		proto.Architecture = &hw.Architecture
	}
	if hw.OS != "" {
		proto.Os = &hw.OS
	}
	if hw.OSVersion != "" {
		proto.OsVersion = &hw.OSVersion
	}

	if hw.ProductName != "" {
		proto.ProductName = &hw.ProductName
	}

	for _, g := range hw.GPUs {
		pg := &nodev1.GPUSpec{
			Model:       g.Model,
			Count:       g.Count,
			MemoryBytes: g.MemoryBytes,
		}
		if g.Architecture != "" {
			pg.GpuArchitecture = &g.Architecture
		}
		proto.Gpus = append(proto.Gpus, pg)
	}

	for _, ic := range hw.Interconnects {
		spec := &nodev1.InterconnectSpec{Device: ic.Device}
		switch ic.Type {
		case "NVLink":
			spec.Details = &nodev1.InterconnectSpec_Nvlink{
				Nvlink: &nodev1.NVLinkDetails{
					ActiveLinks: int32(ic.ActiveLinks),
					Version:     ic.Version,
				},
			}
		case "PCIe":
			spec.Details = &nodev1.InterconnectSpec_Pcie{
				Pcie: &nodev1.PCIeDetails{
					Generation: int32(ic.Generation),
					Width:      int32(ic.Width),
				},
			}
		}
		proto.Interconnects = append(proto.Interconnects, spec)
	}

	return proto
}
