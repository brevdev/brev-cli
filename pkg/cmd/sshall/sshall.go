package sshall

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
)

type SSHAll struct {
	workspaces                 []brevapi.WorkspaceWithMeta
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper
	sshResolver                SSHResolver
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
	}
}

// stringContainsOneOf returns the first match from a list of candidates to
// check if a string contains, otherwise returning false
func stringContainsOneOf(str string, candidates []string) (bool, *string) {
	for _, candidate := range candidates {
		if strings.Contains(str, candidate) {
			return true, &candidate
		}
	}
	return false, nil
}

func workspaceSSHConnectionHealthCheck(w brevapi.WorkspaceWithMeta) bool {
	fmt.Sprintln(w)
	return true
}

func workspaceSSHChannel(w brevapi.WorkspaceWithMeta) chan os.Signal {
	fmt.Sprintln(w)
	return make(chan os.Signal, 1)
}

func (s SSHAll) Run() error {
	// set up error handling for the ssh connections, very much brute force
	// since kubectl was not intended to be used a library like this
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		match, _ := stringContainsOneOf(fmt.Sprint(err), []string{
			"lost connection to pod",
			"error creating stream",
		})

		if match {
			for _, w := range s.workspaces {
				isHealthy := workspaceSSHConnectionHealthCheck(w)
				if !isHealthy {
					close(workspaceSSHChannel(w))

					// TODO: create new close channel and pass it here, also DRY
					go func(workspace brevapi.WorkspaceWithMeta) {
						err := s.portforwardWorkspace(workspace)
						if err != nil {
							// todo have verbose version with trace
							fmt.Printf("%v [workspace=%s]\n", errors.Cause(err), workspace.DNS)
						}
					}(w)
				}
			}
		}
	})

	fmt.Println()
	for _, w := range s.workspaces {
		fmt.Printf("ssh %s\n", w.Name)
		// TODO: create new close channel and pass it here, also DRY
		go func(workspace brevapi.WorkspaceWithMeta) {
			err := s.portforwardWorkspace(workspace)
			if err != nil {
				// todo have verbose version with trace
				fmt.Printf("%v [workspace=%s]\n", errors.Cause(err), workspace.DNS)
			}
		}(w)
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
