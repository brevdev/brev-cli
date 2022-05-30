package huproxyclient

// https://github.com/google/huproxy/blob/master/huproxyclient/client.go

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	huproxy "github.com/google/huproxy/lib"
)

var writeTimeout = 10 * time.Second

type HubProxyStore interface {
	GetAuthTokens() (*entity.AuthTokens, error)
	GetCurrentWorkspaceGroupID() (string, error)
}

func dialError(url string, resp *http.Response, err error) {
	if resp != nil {
		extra := ""
		b, err1 := ioutil.ReadAll(resp.Body)
		if err1 != nil {
			log.Warningf("Failed to read HTTP body: %v", err1)
		}
		extra = "Body:\n" + string(b)
		log.Fatalf("%s: HTTP error: %d %s\n%s", err, resp.StatusCode, resp.Status, extra)

	}
	log.Fatalf("Dial to %q fail: %v", url, err)
}

func Run(url string, store HubProxyStore) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dialer := websocket.Dialer{}
	dialer.TLSClientConfig = new(tls.Config)

	head := map[string][]string{}

	token, err := store.GetAuthTokens()
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	workspaceGroupID, err := store.GetCurrentWorkspaceGroupID()
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	if workspaceGroupID != "" {
		head["X-Workspace-Group-ID"] = []string{workspaceGroupID}
	}

	head["Authorization"] = []string{
		"Bearer " + token.AccessToken,
	}

	conn, resp, err := dialer.Dial(url, head)
	if err != nil {
		dialError(url, resp, err)
	}
	defer conn.Close() //nolint:errcheck // lazy to refactor

	RunProxy(ctx, conn, cancel)

	if ctx.Err() != nil {
		return errors.WrapAndTrace(ctx.Err())
	}
	return nil
}

func RunProxy(ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc) {
	// websocket -> stdout
	go func() {
		for {
			mt, r, err := conn.NextReader()
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return
			}
			if err != nil {
				log.Warn("Workspace disconnect: may be from network failure or workspace was stopped/deleted")
				log.Fatal(err)
			}
			if mt != websocket.BinaryMessage {
				log.Fatal("non-binary websocket message received")
			}
			if _, err := io.Copy(os.Stdout, r); err != nil {
				log.Errorf("Reading from websocket: %v", err)
				cancel()
			}
		}
	}()

	// stdin -> websocket
	// TODO: NextWriter() seems to be broken.
	if err := huproxy.File2WS(ctx, cancel, os.Stdin, conn); err == io.EOF {
		if err1 := conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(writeTimeout)); err1 == websocket.ErrCloseSent {
			_ = ""
		} else if err1 != nil {
			log.Errorf("Error sending 'close' message: %v", err1)
		}
	} else if err != nil {
		log.Errorf("reading from stdin: %v", err)
		cancel()
	}
}
