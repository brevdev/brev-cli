// Go RPC server listening on a Unix socket.
//
// Eli Bendersky [http://eli.thegreenplace.net]
// This code is in the public domain.
package main

import (
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"os"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/vpn"
	"github.com/spf13/afero"
)

const SockAddr = "/tmp/rpc.sock" // todo /var/run/brev/brevtsup.sock

type RPCServerStore interface {
	CopyBin(targetBin string) error
	WriteString(path, data string) error
	RegisterNode(publicKey string) error
	GetOrCreateFile(path string) (afero.File, error)
	GetNetworkAuthKey() (*store.GetAuthKeyResponse, error)
	GetCurrentWorkspaceID() (string, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

type Server struct {
	Store    RPCServerStore
	SockAddr string
}

func NewServer(store RPCServerStore, sockAddr string) Server {
	return Server{
		Store:    store,
		SockAddr: SockAddr,
	}
}

// func check() {
// 	fi, err := os.Stat(SockAddr)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	log.Println("mode", fi.Mode())
// }

func (s Server) TailscaleUp() error {
	vpnd := &vpn.VPNDaemon{
		Store: s.Store,
	}
	err := vpnd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (s Server) Serve() error {
	if err := os.RemoveAll(SockAddr); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	greeter := new(Server)
	rpc.Register(greeter)
	rpc.HandleHTTP()
	l, e := net.Listen("unix", SockAddr)
	if e != nil {
		return breverrors.WrapAndTrace(e)
	}
	if err := os.Chmod(SockAddr, 0o777); err != nil { // todo what are the actual perms we need?
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println("Serving...")
	http.Serve(l, nil)
	return nil
}
