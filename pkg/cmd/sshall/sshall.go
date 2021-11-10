package sshall

import (
	"fmt"
	"math/rand"
	"strconv"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/spf13/cobra"
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

type SSHAll struct{}

func (s SSHAll) Run() error {
	workspaces, err := getActiveWorkspaces() // to inject?
	if err != nil {
		return err
	}

	for _, w := range workspaces {
		id := w.GetID()
		port := getLocalPortForWorkspace(id) // to inject?
		portMapping := makeSSHPortMapping(port)
		err := portforwardWorkspace(id, portMapping) // to inject?
		if err != nil {
			return err
		}
	}

	return nil
}

func getActiveWorkspaces() ([]brev_api.Workspace, error) {
	activeOrg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		return nil, err
	}

	client, err := brev_api.NewCommandClient()
	if err != nil {
		return nil, err
	}
	wss, err := client.GetWorkspaces(activeOrg.ID)
	if err != nil {
		return nil, err
	}

	return wss, nil
}

func getLocalPortForWorkspace(workspaceID string) string {
	minPort := 1024
	maxPort := 65535
	port := rand.Intn(maxPort-minPort) + minPort
	return strconv.Itoa(port)
}

func portforwardWorkspace(workspaceID string, portMapping string) error {
	return nil
}

func makeSSHPortMapping(localPort string) string {
	return fmt.Sprintf("%s:22", localPort)
}
