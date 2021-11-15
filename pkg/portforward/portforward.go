package portforward

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/transport/spdy"

	toolsportforward "k8s.io/client-go/tools/portforward"
)

type PortForwardOptions struct {
	PortForwarder              PortForwarder
	WorkspaceGroupClientMapper k8s.WorkspaceGroupClientMapper

	K8sClient k8s.K8sClient
	K8sAPIURL string

	Namespace string
	PodName   string

	Address      []string
	Ports        []string
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

type PortForwarder interface {
	ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error
}

func NewPortForwardOptions(workspaceGroupClientMapper k8s.WorkspaceGroupClientMapper, portforwarder PortForwarder) *PortForwardOptions {
	p := &PortForwardOptions{
		PortForwarder:              portforwarder,
		WorkspaceGroupClientMapper: workspaceGroupClientMapper,
	}

	p.Address = []string{"localhost"}
	p.StopChannel = make(chan struct{}, 1)
	p.ReadyChannel = make(chan struct{})

	return p
}

func (o *PortForwardOptions) WithWorkspace(workspace brevapi.WorkspaceWithMeta) (*PortForwardOptions, error) {
	o.Namespace = workspace.GetNamespaceName()
	o.PodName = workspace.GetPodName()

	k8sAPIURL, err := o.WorkspaceGroupClientMapper.GetK8sAPIURL(workspace.WorkspaceGroupID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	o.K8sAPIURL = k8sAPIURL

	k8sClient, err := o.WorkspaceGroupClientMapper.GetK8sClient(workspace.WorkspaceGroupID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	o.K8sClient = k8sClient

	if o.PodName == "" {
		return nil, fmt.Errorf("unable to forward port because pod is not found-- workspace may not be running")
	}

	return o, nil
}

func (o *PortForwardOptions) WithPort(port string) *PortForwardOptions {
	o.Ports = []string{port}

	return o
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

	urlStr := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/portforward", o.K8sAPIURL, o.Namespace, o.PodName)

	url, err := url.Parse(urlStr)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err =o.PortForwarder.ForwardPorts("POST", url, o)
	if err != nil {
		breverrors.WrapAndTrace(err)
	}
	return nil
}

type DefaultPortForwarder struct {
	genericclioptions.IOStreams
}

func NewDefaultPortForwarder() *DefaultPortForwarder {
	return &DefaultPortForwarder{
		IOStreams: genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}
}

func (f *DefaultPortForwarder) ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error {
	transport, upgrader, err := spdy.RoundTripperFor(opts.K8sClient.GetK8sRestConfig())
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)
	fw, err := toolsportforward.NewOnAddresses(dialer, opts.Address, opts.Ports, opts.StopChannel, opts.ReadyChannel, f.Out, f.ErrOut)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = fw.ForwardPorts()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
