package portforward

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/terminal"
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
	GetWorkspaceByID(id string) (*brev_api.AllWorkspaceData, error)
	GetWorkspaceByName(name string) (*brev_api.AllWorkspaceData, error)
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

func (o *PortForwardOptions) Complete(cmd *cobra.Command, t *terminal.Terminal, args []string) error {
	workspaceIDOrName := args[0]

	workspace, err := o.ResourceResolver.GetWorkspaceByID(workspaceIDOrName)
	if err != nil {
		wsByName, err2 := o.ResourceResolver.GetWorkspaceByName(workspaceIDOrName)
		if err2 != nil {
			return err2
		} else {
			workspace = wsByName
		}
	}
	if workspace == nil {
		return fmt.Errorf("workspace with id or name %s does not exist", workspaceIDOrName)
	}
	// handle diff org than settings

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

	urlStr := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/portforward", config.GlobalConfig.GetKubeAPIURL(), o.Namespace, o.PodName)

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
