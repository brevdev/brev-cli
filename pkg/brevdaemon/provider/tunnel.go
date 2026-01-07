package provider

// TunnelPortMapping describes how a local port is exposed through the tunnel.
type TunnelPortMapping struct {
	LocalPort  int
	RemotePort int
	Protocol   string
}
