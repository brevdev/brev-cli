package sshall

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewCmdSSHAll() *cobra.Command {
	opts := sshAllOptions{}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "ssh-all",
		DisableFlagsInUseLine: true,
		Short:                 "run all workspace ssh connections",
		Long:                  "run ssh connections for all running workspaces",
		Example:               "brev ssh-all",
		Args:                  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate())
			cmdutil.CheckErr(opts.RunSSHAll())
		},
	}
	return cmd
}

type sshAllOptions struct {
	sshAll *SSHAll
}

func (s *sshAllOptions) Complete(cmd *cobra.Command, args []string) error {
	k8sClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper() // to resolve
	if err != nil {
		return err
	}

	client, err := brev_api.NewCommandClient() // to resolve
	if err != nil {
		return err
	}

	sshResolver := NewRandomPortSSHResolver(client)
	s.sshAll = NewSSHAll(k8sClientMapper, sshResolver)
	return nil
}

func (s sshAllOptions) Validate() error {
	return nil
}

func (s sshAllOptions) RunSSHAll() error {
	return s.sshAll.Run()
}

type SSHAll struct {
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper
	sshResolver                SSHResolver
}

type SSHResolver interface {
	GetWorkspaces() ([]brev_api.WorkspaceWithMeta, error)
	GetConfiguredWorkspacePort(workspace brev_api.Workspace) (string, error)
}

func NewSSHAll(
	workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper,
	sshResolver SSHResolver,

) *SSHAll {
	return &SSHAll{
		workspaceGroupClientMapper: workspaceGroupClientMapper,
		sshResolver:                sshResolver,
	}
}

func (s SSHAll) Run() error {
	workspaces, err := s.sshResolver.GetWorkspaces()
	if err != nil {
		return err
	}

	for _, w := range workspaces {
		port, err := s.sshResolver.GetConfiguredWorkspacePort(w.Workspace)
		if err != nil {
			return err
		}
		portMapping := makeSSHPortMapping(port)
		go func() {
			err := s.portforwardWorkspace(w, portMapping)
			if err != nil {
				fmt.Printf("%v\n", err)
			}
		}()
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)
	<-signals

	return nil
}

func (s SSHAll) portforwardWorkspace(workspace brev_api.WorkspaceWithMeta, portMapping string) error {
	dpf := portforward.NewDefaultPortForwarder()
	pf := portforward.NewPortForwardOptions(
		s.workspaceGroupClientMapper,
		dpf,
	)

	_, err := pf.WithWorkspace(workspace)
	if err != nil {
		return err
	}
	pf.WithPort(portMapping)

	err = pf.RunPortforward()
	if err != nil {
		return err
	}

	return nil
}

type (
	RandomSSHResolver struct {
		WorkspaceResolver WorkspaceResolver
	}
	WorkspaceResolver interface {
		GetMyWorkspaces(orgID string) ([]brev_api.Workspace, error)
		GetWorkspaceMetaData(wsID string) (*brev_api.WorkspaceMetaData, error)
	}
)

func NewRandomPortSSHResolver(workspaceResolver WorkspaceResolver) *RandomSSHResolver {
	return &RandomSSHResolver{
		WorkspaceResolver: workspaceResolver,
	}
}

func (r RandomSSHResolver) GetWorkspaces() ([]brev_api.WorkspaceWithMeta, error) {
	activeOrg, err := brev_api.GetActiveOrgContext(files.AppFs) // to inject
	if err != nil {
		return nil, err
	}

	wss, err := r.WorkspaceResolver.GetMyWorkspaces(activeOrg.ID)
	if err != nil {
		return nil, err
	}

	var workspacesWithMeta []brev_api.WorkspaceWithMeta
	for _, w := range wss {
		wmeta, err := r.WorkspaceResolver.GetWorkspaceMetaData(w.ID)
		if err != nil {
			return nil, err
		}

		workspaceWithMeta := brev_api.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: w}
		workspacesWithMeta = append(workspacesWithMeta, workspaceWithMeta)
	}

	return workspacesWithMeta, nil
}

func (r RandomSSHResolver) GetConfiguredWorkspacePort(workspace brev_api.Workspace) (string, error) {
	minPort := 1024
	maxPort := 65535
	port := rand.Intn(maxPort-minPort) + minPort
	return strconv.Itoa(port), nil
}

func makeSSHPortMapping(localPort string) string {
	return fmt.Sprintf("%s:22", localPort)
}
