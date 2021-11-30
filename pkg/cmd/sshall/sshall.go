package sshall

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"strconv"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/runtime"
)

type (
	connectionMap map[brevapi.WorkspaceWithMeta]chan struct{}
	retrymap      map[brevapi.WorkspaceWithMeta]int
)

type SSHAll struct {
	workspaces                 []brevapi.WorkspaceWithMeta
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper
	sshResolver                SSHResolver
	workspaceConnections       connectionMap
	retries                    retrymap
}

type SSHResolver interface {
	GetConfiguredWorkspacePort(workspace brevapi.Workspace) (string, error)
}

func NewSSHAll(
	workspaces []brevapi.WorkspaceWithMeta,
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper,
	sshResolver SSHResolver,

) *SSHAll {
	return &SSHAll{
		workspaces:                 workspaces,
		workspaceGroupClientMapper: workspaceGroupClientMapper,
		sshResolver:                sshResolver,
		workspaceConnections:       make(connectionMap),
		retries:                    make(retrymap),
	}
}

func workspaceSSHConnectionHealthCheck(w brevapi.WorkspaceWithMeta) (bool, error) {
	var hostKey ssh.PublicKey
	// A public key may be used to authenticate against the remote
	// 	var hostKey ssh.PublicKey
	// A public key may be used to authenticate against the remote
	// server by using an unencrypted PEM-encoded private key file.
	//
	// If you have an encrypted private key, the crypto/x509 package
	// can be used to decrypt it.
	key, err := ioutil.ReadFile(files.GetSSHPrivateKeyFilePath())
	if err != nil {
		return false, breverrors.WrapAndTrace(err, "unable to read private key: %v")
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return false, breverrors.WrapAndTrace(err, "unable to parse private key: %v")
	}

	config := &ssh.ClientConfig{
		User: "brev",
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.FixedHostKey(hostKey),
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", w.DNS, config)
	if err != nil {
		return false, breverrors.WrapAndTrace(err, "unable to connect: %v")
	}
	err = client.Close()
	if err != nil {
		return true, breverrors.WrapAndTrace(err)
	}
	return true, nil
}

func (s SSHAll) runPortForwardWorkspace(workspace brevapi.WorkspaceWithMeta) {
	err := s.portforwardWorkspace(workspace)
	if err != nil {
		// todo have verbose version with trace
		fmt.Printf("%v [workspace=%s]\n", errors.Cause(err), workspace.DNS)
	}
}

func (s SSHAll) Run() error {
	// set up error handling for the ssh connections, very much brute force
	// since kubectl was not intended to be used a library like this
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		for _, w := range s.workspaces {
			isHealthy, _ := workspaceSSHConnectionHealthCheck(w)
			if !isHealthy {
				close(s.workspaceConnections[w])
				if s.retries[w] > 0 {
					s.retries[w]--
					go s.runPortForwardWorkspace(w)
				}
			}
		}
	})

	if len(s.workspaces) == 0 {
		// todo set one for the user
		return breverrors.WrapAndTrace(errors.New("please set an active workspace"))
	}

	fmt.Println()
	for _, w := range s.workspaces {
		fmt.Printf("ssh %s\n", w.DNS)
		s.retries[w] = 3 // TODO magic number
		go s.runPortForwardWorkspace(w)
	}
	fmt.Println()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)
	<-signals

	return nil
}

func (s SSHAll) portforwardWorkspace(workspace brevapi.WorkspaceWithMeta) error {
	port, err := s.sshResolver.GetConfiguredWorkspacePort(workspace.Workspace)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if port == "" {
		return fmt.Errorf("port not found")
	}
	portMapping := makeSSHPortMapping(port)
	err = s.portforwardWorkspaceAtPort(workspace, portMapping)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (s SSHAll) portforwardWorkspaceAtPort(workspace brevapi.WorkspaceWithMeta, portMapping string) error {
	dpf := portforward.NewDefaultPortForwarder()
	pf := portforward.NewPortForwardOptions(
		s.workspaceGroupClientMapper,
		dpf,
	)

	s.workspaceConnections[workspace] = pf.StopChannel
	_, err := pf.WithWorkspace(workspace)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	pf.WithPort(portMapping)

	err = pf.RunPortforward()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

type (
	RandomSSHResolver struct {
		WorkspaceResolver WorkspaceResolver
	}
	WorkspaceResolver interface {
		GetMyWorkspaces(orgID string) ([]brevapi.Workspace, error)
		GetWorkspaceMetaData(wsID string) (*brevapi.WorkspaceMetaData, error)
	}
)

func NewRandomPortSSHResolver(workspaceResolver WorkspaceResolver) *RandomSSHResolver {
	return &RandomSSHResolver{
		WorkspaceResolver: workspaceResolver,
	}
}

func (r RandomSSHResolver) GetWorkspaces() ([]brevapi.WorkspaceWithMeta, error) {
	activeOrg, err := brevapi.GetActiveOrgContext(files.AppFs) // to inject
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	wss, err := r.WorkspaceResolver.GetMyWorkspaces(activeOrg.ID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var workspacesWithMeta []brevapi.WorkspaceWithMeta
	for _, w := range wss {
		wmeta, err := r.WorkspaceResolver.GetWorkspaceMetaData(w.ID)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}

		workspaceWithMeta := brevapi.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: w}
		workspacesWithMeta = append(workspacesWithMeta, workspaceWithMeta)
	}

	return workspacesWithMeta, nil
}

func (r RandomSSHResolver) GetConfiguredWorkspacePort(_ brevapi.Workspace) (string, error) {
	minPort := 1024
	maxPort := 65535
	port := rand.Intn(maxPort-minPort) + minPort // #nosec no need to by cryptographically secure
	return strconv.Itoa(port), nil
}

func makeSSHPortMapping(localPort string) string {
	return fmt.Sprintf("%s:22", localPort)
}
