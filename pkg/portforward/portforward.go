package portforward

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/transport/spdy"

	toolsportforward "k8s.io/client-go/tools/portforward"
)

type PortForwardOptions struct {
	PortForwarder    PortForwarder
	K8sClient        k8s.K8sClient
	ResourceResolver ResourceResolver

	Namespace string
	PodName   string

	Address      []string
	Ports        []string
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

type ResourceResolver interface {
	GetWorkspaceByID(id string) (*brev_api.WorkspaceMetaData, error)
}

type PortForwarder interface {
	ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error
}

func NewPortForwardOptions(portForwardHelpers k8s.K8sClient, workspaceResolver ResourceResolver, portforwarder PortForwarder) *PortForwardOptions {
	return &PortForwardOptions{
		PortForwarder:    portforwarder,
		K8sClient:        portForwardHelpers,
		ResourceResolver: workspaceResolver,
	}
}

func (o *PortForwardOptions) Complete(cmd *cobra.Command, args []string) error {
	workspaceID := args[0]

	workspace, err := o.ResourceResolver.GetWorkspaceByID(workspaceID)
	if err != nil {
		return err
	}
	if workspace == nil {
		return fmt.Errorf("workspace with id %s does not exist", workspaceID)
	}

	o.Namespace = workspace.GetNamespaceName()
	o.PodName = workspace.GetPodName()

	if o.PodName == "" {
		return fmt.Errorf("unable to forward port because pod is not found-- workspace may not be running")
	}

	o.Address = []string{"localhost"}
	o.Ports = []string{"2222:22"} // TODO override from args

	o.StopChannel = make(chan struct{}, 1)
	o.ReadyChannel = make(chan struct{})

	return nil
}

func (o PortForwardOptions) RunPortforward() error {
	// cmd := portforward.NewCmdPortForward(tf, streams) // This command is useful to have around to go to def of kubectl cmd

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	go func() {
		<-signals
		if o.StopChannel != nil {
			close(o.StopChannel)
		}
	}()

	urlStr := fmt.Sprintf("https://api.k8s.brevstack.com/api/v1/namespaces/%s/pods/%s/portforward", o.Namespace, o.PodName)

	url, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	return o.PortForwarder.ForwardPorts("POST", url, o)
}

type DefaultPortForwarder struct {
	genericclioptions.IOStreams
}

func (f *DefaultPortForwarder) ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error {
	transport, upgrader, err := spdy.RoundTripperFor(opts.K8sClient.GetK8sRestConfig())
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)
	fw, err := toolsportforward.NewOnAddresses(dialer, opts.Address, opts.Ports, opts.StopChannel, opts.ReadyChannel, f.Out, f.ErrOut)
	if err != nil {
		return err
	}

	err = fw.ForwardPorts()
	if err != nil {
		return err
	}
	return nil
}
