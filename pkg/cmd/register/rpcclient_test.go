package register

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
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

func Test_toProtoNodeSpec(t *testing.T) {
	cpuCount := int32(12)
	ramBytes := int64(137438953472)
	memBytes := int64(137438953472)

	local := &HardwareProfile{
		GPUs: []GPU{
			{Model: "NVIDIA GB10", Architecture: "Blackwell", Count: 2, MemoryBytes: &memBytes},
		},
		RAMBytes:     &ramBytes,
		CPUCount:     &cpuCount,
		Architecture: "arm64",
		Storage: []StorageDevice{
			{Name: "nvme0n1", StorageBytes: 500107862016, StorageType: "SSD"},
		},
		OS:          "Ubuntu",
		OSVersion:   "24.04",
		ProductName: "DGX Spark",
		Interconnects: []Interconnect{
			{Type: "NVLink", Device: "GPU 0", ActiveLinks: 4, Version: 4},
		},
	}

	proto := toProtoNodeSpec(local)

	if proto.GetCpuCount() != 12 {
		t.Errorf("expected CpuCount 12, got %d", proto.GetCpuCount())
	}
	if proto.GetRamBytes() != 137438953472 {
		t.Errorf("expected RamBytes, got %d", proto.GetRamBytes())
	}
	if proto.GetArchitecture() != "arm64" {
		t.Errorf("expected arm64, got %s", proto.GetArchitecture())
	}
	if proto.GetOs() != "Ubuntu" {
		t.Errorf("expected Ubuntu, got %s", proto.GetOs())
	}
	if proto.GetOsVersion() != "24.04" {
		t.Errorf("expected 24.04, got %s", proto.GetOsVersion())
	}
	if len(proto.GetStorage()) != 1 {
		t.Fatalf("expected 1 storage entry, got %d", len(proto.GetStorage()))
	}
	if proto.GetStorage()[0].GetStorageBytes() != 500107862016 {
		t.Errorf("expected StorageBytes 500107862016, got %d", proto.GetStorage()[0].GetStorageBytes())
	}
	if proto.GetStorage()[0].GetStorageType() != "SSD" {
		t.Errorf("expected SSD, got %s", proto.GetStorage()[0].GetStorageType())
	}
	if len(proto.GetGpus()) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(proto.GetGpus()))
	}
	gpu := proto.GetGpus()[0]
	if gpu.GetModel() != "NVIDIA GB10" {
		t.Errorf("expected NVIDIA GB10, got %s", gpu.GetModel())
	}
	if gpu.GetCount() != 2 {
		t.Errorf("expected count 2, got %d", gpu.GetCount())
	}
	if gpu.GetMemoryBytes() != 137438953472 {
		t.Errorf("expected memory bytes, got %d", gpu.GetMemoryBytes())
	}
}

func Test_toProtoNodeSpec_PCIeInterconnect(t *testing.T) {
	local := &HardwareProfile{
		Architecture: "amd64",
		Interconnects: []Interconnect{
			{Type: "PCIe", Device: "GPU 0", Generation: 4, Width: 16},
		},
	}

	proto := toProtoNodeSpec(local)

	if len(proto.GetInterconnects()) != 1 {
		t.Fatalf("expected 1 interconnect, got %d", len(proto.GetInterconnects()))
	}
	ic := proto.GetInterconnects()[0]
	if ic.GetDevice() != "GPU 0" {
		t.Errorf("expected device 'GPU 0', got %s", ic.GetDevice())
	}
	pcie := ic.GetPcie()
	if pcie == nil {
		t.Fatal("expected PCIe details, got nil")
	}
	if pcie.GetGeneration() != 4 {
		t.Errorf("expected PCIe Gen 4, got %d", pcie.GetGeneration())
	}
	if pcie.GetWidth() != 16 {
		t.Errorf("expected PCIe Width 16, got %d", pcie.GetWidth())
	}
}

func Test_toProtoNodeSpec_Nil(t *testing.T) {
	if toProtoNodeSpec(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

func Test_toProtoNodeSpec_MinimalFields(t *testing.T) {
	local := &HardwareProfile{
		Architecture: "amd64",
	}
	proto := toProtoNodeSpec(local)
	if proto.GetArchitecture() != "amd64" {
		t.Errorf("expected amd64, got %s", proto.GetArchitecture())
	}
	if proto.RamBytes != nil {
		t.Error("expected nil RamBytes")
	}
	if proto.CpuCount != nil {
		t.Error("expected nil CpuCount")
	}
	if len(proto.GetGpus()) != 0 {
		t.Error("expected no GPUs")
	}
}

// fakeNodeService implements the server side of ExternalNodeService for testing.
type fakeNodeService struct {
	nodev1connect.UnimplementedExternalNodeServiceHandler
	addNodeFn            func(*nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error)
	removeNodeFn         func(*nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error)
	getNodeFn            func(*nodev1.GetNodeRequest) (*nodev1.GetNodeResponse, error)
	openPortFn           func(*nodev1.OpenPortRequest) (*nodev1.OpenPortResponse, error)
	grantNodeSSHAccessFn func(*nodev1.GrantNodeSSHAccessRequest) (*nodev1.GrantNodeSSHAccessResponse, error)
}

func (f *fakeNodeService) AddNode(_ context.Context, req *connect.Request[nodev1.AddNodeRequest]) (*connect.Response[nodev1.AddNodeResponse], error) {
	resp, err := f.addNodeFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (f *fakeNodeService) RemoveNode(_ context.Context, req *connect.Request[nodev1.RemoveNodeRequest]) (*connect.Response[nodev1.RemoveNodeResponse], error) {
	resp, err := f.removeNodeFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (f *fakeNodeService) GetNode(_ context.Context, req *connect.Request[nodev1.GetNodeRequest]) (*connect.Response[nodev1.GetNodeResponse], error) {
	if f.getNodeFn == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, nil)
	}
	resp, err := f.getNodeFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (f *fakeNodeService) OpenPort(_ context.Context, req *connect.Request[nodev1.OpenPortRequest]) (*connect.Response[nodev1.OpenPortResponse], error) {
	if f.openPortFn == nil {
		return connect.NewResponse(&nodev1.OpenPortResponse{
			Port: &nodev1.Port{
				PortId:     "port_ssh",
				Protocol:   req.Msg.GetProtocol(),
				PortNumber: req.Msg.GetPortNumber(),
				ServerPort: req.Msg.GetPortNumber(),
			},
		}), nil
	}
	resp, err := f.openPortFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (f *fakeNodeService) GrantNodeSSHAccess(_ context.Context, req *connect.Request[nodev1.GrantNodeSSHAccessRequest]) (*connect.Response[nodev1.GrantNodeSSHAccessResponse], error) {
	if f.grantNodeSSHAccessFn == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, nil)
	}
	resp, err := f.grantNodeSSHAccessFn(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func Test_NewNodeServiceClient_AddNode(t *testing.T) {
	svc := &fakeNodeService{
		addNodeFn: func(req *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			if req.GetOrganizationId() != "org_123" {
				t.Errorf("unexpected org ID: %s", req.GetOrganizationId())
			}
			if req.GetName() != "My Spark" {
				t.Errorf("unexpected name: %s", req.GetName())
			}
			return &nodev1.AddNodeResponse{
				ExternalNode: &nodev1.ExternalNode{
					ExternalNodeId: "unode_abc",
					OrganizationId: "org_123",
					Name:           req.GetName(),
					DeviceId:       req.GetDeviceId(),
				},
			}, nil
		},
	}

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	client := NewNodeServiceClient(&mockTokenProvider{token: "tok"}, server.URL)

	resp, err := client.AddNode(context.Background(), connect.NewRequest(&nodev1.AddNodeRequest{
		OrganizationId: "org_123",
		Name:           "My Spark",
		DeviceId:       "dev-uuid",
		NodeSpec:       &nodev1.NodeSpec{Architecture: strPtr("arm64")},
	}))
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}
	if resp.Msg.GetExternalNode().GetExternalNodeId() != "unode_abc" {
		t.Errorf("unexpected node ID: %s", resp.Msg.GetExternalNode().GetExternalNodeId())
	}
}

func Test_NewNodeServiceClient_AddNode_ServerError(t *testing.T) {
	svc := &fakeNodeService{
		addNodeFn: func(_ *nodev1.AddNodeRequest) (*nodev1.AddNodeResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, nil)
		},
	}

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	client := NewNodeServiceClient(&mockTokenProvider{token: "tok"}, server.URL)

	_, err := client.AddNode(context.Background(), connect.NewRequest(&nodev1.AddNodeRequest{
		OrganizationId: "org_123",
		Name:           "Test",
		DeviceId:       "dev",
	}))
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func Test_NewNodeServiceClient_RemoveNode(t *testing.T) {
	svc := &fakeNodeService{
		removeNodeFn: func(req *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			if req.GetExternalNodeId() != "unode_abc" {
				t.Errorf("unexpected node ID: %s", req.GetExternalNodeId())
			}
			return &nodev1.RemoveNodeResponse{}, nil
		},
	}

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	client := NewNodeServiceClient(&mockTokenProvider{token: "tok"}, server.URL)

	_, err := client.RemoveNode(context.Background(), connect.NewRequest(&nodev1.RemoveNodeRequest{
		ExternalNodeId: "unode_abc",
	}))
	if err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}
}

func Test_NewNodeServiceClient_RemoveNode_ServerError(t *testing.T) {
	svc := &fakeNodeService{
		removeNodeFn: func(_ *nodev1.RemoveNodeRequest) (*nodev1.RemoveNodeResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, nil)
		},
	}

	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	client := NewNodeServiceClient(&mockTokenProvider{token: "tok"}, server.URL)

	_, err := client.RemoveNode(context.Background(), connect.NewRequest(&nodev1.RemoveNodeRequest{
		ExternalNodeId: "unode_missing",
	}))
	if err == nil {
		t.Fatal("expected error for not found response")
	}
}

func strPtr(s string) *string { return &s }
