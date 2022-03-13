// Go RPC server listening on a Unix socket.
//
// Eli Bendersky [http://eli.thegreenplace.net]
// This code is in the public domain.
package main

import (
	"fmt"
	"log"
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

// todo /var/run/brev/brevvpnd.sock
const SockAddr = "/tmp/rpc.sock"

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
	Store RPCServerStore
}

func check() {
	fi, err := os.Stat(SockAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("mode", fi.Mode())
}

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

func main() {
	if err := os.RemoveAll(SockAddr); err != nil {
		log.Fatal(err)
	}

	greeter := new(Server)
	rpc.Register(greeter)
	rpc.HandleHTTP()
	l, e := net.Listen("unix", SockAddr)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	if err := os.Chmod(SockAddr, 0o777); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Serving...")
	http.Serve(l, nil)
}
