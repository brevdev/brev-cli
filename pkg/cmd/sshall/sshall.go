package sshall

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/cmd/link"
	"github.com/brevdev/brev-cli/pkg/cmd/ls"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type sshAllOptions struct{}

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

func (s *sshAllOptions) Complete(cmd *cobra.Command, args []string) error {
	return nil
}

func (s sshAllOptions) Validate() error {
	return nil
}

func (s sshAllOptions) RunSSHAll() error {
	return nil
}

func RunSSHAll() error {
	workspaces, err := getUserActiveWorkspaces() // to inject?
	if err != nil {
		return err
	}

	for _, w := range workspaces {
		id := w.GetID()
		port := getLocalPortForWorkspace(id) // to inject?
		portMapping := makeSSHPortMapping(port)
		go func() {
			err := portforwardWorkspace(id, portMapping) // to inject?
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

func getUserActiveWorkspaces() ([]brev_api.Workspace, error) {
	activeOrg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		return nil, err
	}

	wss, err := ls.GetAllWorkspaces(activeOrg.ID)
	if err != nil {
		return nil, err
	}
	_, userWorkspaces := ls.GetSortedUserWorkspaces(wss)

	return userWorkspaces, nil
}

func getLocalPortForWorkspace(workspaceID string) string {
	minPort := 1024
	maxPort := 65535
	port := rand.Intn(maxPort-minPort) + minPort
	return strconv.Itoa(port)
}

func portforwardWorkspace(workspaceID string, portMapping string) error {
	k8sClientConfig, err := link.NewRemoteK8sClientConfig()
	if err != nil {
		return err
	}
	k8sClient := k8s.NewDefaultClient(k8sClientConfig)

	pf := portforward.NewPortForwardOptions(
		k8sClient,
		link.WorkspaceResolver{},
		&portforward.DefaultPortForwarder{
			IOStreams: genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		},
	)

	_, err = pf.WithWorkspace(workspaceID)
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

func makeSSHPortMapping(localPort string) string {
	return fmt.Sprintf("%s:22", localPort)
}
