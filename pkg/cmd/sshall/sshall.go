package sshall

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"
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

func (s SSHAll) Run() error {
	fmt.Println()
	for _, w := range s.workspaces {
		fmt.Printf("ssh %s\n", w.DNS)
		go func(workspace brevapi.WorkspaceWithMeta) {
			err := s.portforwardWorkspace(workspace)
			if err != nil {
				fmt.Printf("%v [workspace=%s]\n", err, workspace.DNS)
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
