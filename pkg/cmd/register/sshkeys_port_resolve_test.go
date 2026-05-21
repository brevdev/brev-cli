package register

import (
	"context"
	"net/http/httptest"
	"testing"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/terminal"
)

type mockPortSelector struct {
	choices []string
	idx     int
}

func (m *mockPortSelector) Select(_ string, items []string) string {
	if m.idx < len(m.choices) {
		ch := m.choices[m.idx]
		m.idx++
		return ch
	}
	if len(items) > 0 {
		return items[0]
	}
	return ""
}

type portOpenTestStore struct{ token string }

func (p portOpenTestStore) GetAccessToken() (string, error) { return p.token, nil }

type openPortCapture struct {
	number   int32
	protocol nodev1.PortProtocol
}

func startPortOpenTestServer(t *testing.T) (mockNodeClientFactory, *openPortCapture) {
	t.Helper()
	cap := &openPortCapture{}
	svc := &fakeNodeService{
		openPortFn: func(req *nodev1.OpenPortRequest) (*nodev1.OpenPortResponse, error) {
			cap.number = req.GetPortNumber()
			cap.protocol = req.GetProtocol()
			return &nodev1.OpenPortResponse{Port: &nodev1.Port{
				PortId:     "port_new",
				Protocol:   req.GetProtocol(),
				PortNumber: req.GetPortNumber(),
				ServerPort: 22,
			}}, nil
		},
	}
	_, handler := nodev1connect.NewExternalNodeServiceHandler(svc)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return mockNodeClientFactory{serverURL: server.URL}, cap
}

func TestResolveSSHAccessPort_noPortsOpensNew(t *testing.T) {
	SetTestSSHPort(2222)
	defer ClearTestSSHPort()

	clients, cap := startPortOpenTestServer(t)
	portID, err := ResolveSSHAccessPort(
		context.Background(),
		terminal.New(),
		&mockPortSelector{},
		clients,
		portOpenTestStore{token: "tok"},
		&DeviceRegistration{ExternalNodeID: "unode_abc", OrgID: "org_1"},
		&nodev1.ExternalNode{ExternalNodeId: "unode_abc"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if portID != "port_new" {
		t.Fatalf("got port id %q", portID)
	}
	if cap.number != 2222 {
		t.Fatalf("open port number = %d, want 2222", cap.number)
	}
	if cap.protocol != nodev1.PortProtocol_PORT_PROTOCOL_TCP {
		t.Fatalf("protocol = %v, want TCP", cap.protocol)
	}
}

func TestResolveSSHAccessPort_useExisting(t *testing.T) {
	ports := []*nodev1.Port{
		{PortId: "port_a", PortNumber: 11640, ServerPort: 22},
		{PortId: "port_b", PortNumber: 8080, ServerPort: 8080},
	}
	sel := &mockPortSelector{choices: []string{PortChoiceUseExisting, "11640->22"}}

	portID, err := ResolveSSHAccessPort(
		context.Background(),
		terminal.New(),
		sel,
		mockNodeClientFactory{serverURL: "http://unused"},
		portOpenTestStore{token: "tok"},
		&DeviceRegistration{ExternalNodeID: "unode_abc", OrgID: "org_1"},
		&nodev1.ExternalNode{Ports: ports},
	)
	if err != nil {
		t.Fatal(err)
	}
	if portID != "port_a" {
		t.Fatalf("got port id %q, want port_a", portID)
	}
}

func TestResolveSSHAccessPort_openNewWhenPortsExist(t *testing.T) {
	SetTestSSHPort(2222)
	defer ClearTestSSHPort()

	clients, cap := startPortOpenTestServer(t)
	sel := &mockPortSelector{choices: []string{PortChoiceOpenNew}}
	portID, err := ResolveSSHAccessPort(
		context.Background(),
		terminal.New(),
		sel,
		clients,
		portOpenTestStore{token: "tok"},
		&DeviceRegistration{ExternalNodeID: "unode_abc", OrgID: "org_1"},
		&nodev1.ExternalNode{Ports: []*nodev1.Port{{PortId: "port_existing", PortNumber: 11640, ServerPort: 22}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if portID != "port_new" {
		t.Fatalf("got port id %q", portID)
	}
	if cap.number != 2222 {
		t.Fatalf("open port number = %d, want 2222", cap.number)
	}
}
