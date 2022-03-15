// Go RPC server listening on a Unix socket.
//
// Eli Bendersky [http://eli.thegreenplace.net]
// This code is in the public domain.
package rpcserver

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
	rpc.Register(server)
	rpc.HandleHTTP()
	l, e := net.Listen("unix", s.SockAddr)
	if e != nil {
		return breverrors.WrapAndTrace(e)
	}
	if err := os.Chmod(s.SockAddr, 0o777); err != nil { // todo what are the actual perms we need?
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println("Serving...")
	http.Serve(l, nil)
	return nil
}
