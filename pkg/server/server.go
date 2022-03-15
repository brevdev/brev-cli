package server

// Go RPC server listening on a Unix socket.
//
// Eli Bendersky [http://eli.thegreenplace.net]
// This code is in the public domain.

import (
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"os"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/vpn"
)

type RPCServerStore interface {
	vpn.ServiceMeshStore
}

type Server struct {
	Store    RPCServerStore
	SockAddr string
}

func NewServer(store RPCServerStore, sockAddr string) Server {
	return Server{
		Store:    store,
		SockAddr: sockAddr,
	}
}

type Client struct {
	client *rpc.Client
}

func NewClient(sockAddr string) (*Client, error) {
	client, err := rpc.DialHTTP("unix", sockAddr)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &Client{client}, nil
}

func (s Server) ConfigureVPN() error {
	err := vpn.ConfigureVPN(s.Store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (c Client) ConfigureVPN() error {
	err := c.client.Call("Server.ConfigureVPN", nil, nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (s Server) Serve() error {
	if err := os.RemoveAll(s.SockAddr); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	server := new(Server)
	err := rpc.Register(server)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	rpc.HandleHTTP()
	l, err := net.Listen("unix", s.SockAddr)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = os.Chmod(s.SockAddr, 0o666) //nolint:gosec // need to allow other users to write to socket
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println("Serving...")
	err = http.Serve(l, nil)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
