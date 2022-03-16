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

type ServerStore interface {
	vpn.ServiceMeshStore
}

type RPCServer struct {
	Store ServerStore
}

func NewRPCServer(store ServerStore) RPCServer {
	return RPCServer{
		Store: store,
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

func (s RPCServer) ConfigureVPN(_ *string, _ *string) error {
	err := vpn.ConfigureVPN(s.Store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (c Client) ConfigureVPN() error {
	in := ""
	out := ""
	err := c.client.Call("RpcServer.ConfigureVPN", &in, &out)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

type Server struct {
	SockAddr string
	Store    ServerStore
}

func NewServer(sockAddr string, store ServerStore) Server {
	return Server{SockAddr: sockAddr, Store: store}
}

func (s Server) Serve() error {
	if err := os.RemoveAll(s.SockAddr); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	server := NewRPCServer(s.Store)
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
