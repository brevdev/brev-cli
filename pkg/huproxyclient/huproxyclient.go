package huproxyclient

// https://github.com/google/huproxy/blob/master/huproxyclient/client.go

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	huproxy "github.com/google/huproxy/lib"
)

var (
	writeTimeout = flag.Duration("write_timeout", 10*time.Second, "Write timeout")
	basicAuth    = flag.String("auth", "", "HTTP Basic Auth in @<filename> or <username>:<password> format.")
	certFile     = flag.String("cert", "", "Certificate Auth File")
	keyFile      = flag.String("key", "", "Certificate Key File")
	verbose      = flag.Bool("verbose", false, "Verbose.")
	insecure     = flag.Bool("insecure_conn", false, "Skip certificate validation")
)

func secretString(s string) (string, error) {
	if strings.HasPrefix(s, "@") {
		fn := s[1:]
		st, err := os.Stat(fn)
		if err != nil {
			return "", errors.WrapAndTrace(err)
		}
		p := st.Mode() & os.ModePerm
		if p&0o177 > 0 {
			return "", fmt.Errorf("valid permissions for %q is %0o, was %0o", fn, 0o600, p)
		}
		b, err := ioutil.ReadFile(fn) //nolint:gosec // want to allow variable read in since lives on client
		return strings.TrimSpace(string(b)), err
	}
	return s, nil
}

func dialError(url string, resp *http.Response, err error) {
	if resp != nil {
		extra := ""
		if *verbose {
			b, err1 := ioutil.ReadAll(resp.Body)
			if err1 != nil {
				log.Warningf("Failed to read HTTP body: %v", err1)
			}
			extra = "Body:\n" + string(b)
		}
		log.Fatalf("%s: HTTP error: %d %s\n%s", err, resp.StatusCode, resp.Status, extra)

	}
	log.Fatalf("Dial to %q fail: %v", url, err)
}

// used to be main
func Do() {
	flag.Parse()

	if flag.NArg() != 1 {
		log.Fatalf("Want exactly one arg")
	}
	url := flag.Arg(0)

	if *verbose {
		log.Infof("huproxyclient %s", huproxy.Version)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dialer := websocket.Dialer{}
	dialer.TLSClientConfig = new(tls.Config)
	if *insecure {
		dialer.TLSClientConfig.InsecureSkipVerify = true
	}
	head := map[string][]string{}

	// Add basic auth.
	if *basicAuth != "" {
		ss, err := secretString(*basicAuth)
		if err != nil {
			log.Panicf("Error reading secret string %q: %v", *basicAuth, err)
		}
		a := base64.StdEncoding.EncodeToString([]byte(ss))
		head["Authorization"] = []string{
			"Basic " + a,
		}
	}

	// Load client cert
	if *certFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Panic(err)
		}

		dialer.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}

	conn, resp, err := dialer.Dial(url, head)
	if err != nil {
		dialError(url, resp, err)
	}
	defer conn.Close() //nolint:errcheck // lazy to refactor

	DoProxy(ctx, conn, cancel)

	if ctx.Err() != nil {
		log.Panic(ctx.Err())
	}
}

func DoProxy(ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc) {
	// websocket -> stdout
	go func() {
		for {
			mt, r, err := conn.NextReader()
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return
			}
			if err != nil {
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
			time.Now().Add(*writeTimeout)); err1 == websocket.ErrCloseSent {
			_ = ""
		} else if err1 != nil {
			log.Errorf("Error sending 'close' message: %v", err1)
		}
	} else if err != nil {
		log.Errorf("reading from stdin: %v", err)
		cancel()
	}
}
