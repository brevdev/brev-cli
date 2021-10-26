package portforward

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubectl/pkg/cmd/portforward"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolsportforward "k8s.io/client-go/tools/portforward"
)

type PortForwardOptions struct {
	PortForwarder     PortForwarder
	PortForwardClient PortForwardClient
	WorkspaceResolver

	Namespace string
	PodName   string

	Address      []string
	Ports        []string
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

type PortForwardClient interface {
	GetK8sClient() *kubernetes.Clientset
	GetK8sRestConfig() *rest.Config
}

type WorkspaceResolver interface {
	GetWorkspaceByID(id string) (Workspace, error)
}

type Workspace interface {
	GetPodName() string
	GetNamespaceName() string
}

type PortForwarder interface {
	ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error
}

func NewPortForwardOptions(portForwardHelpers PortForwardClient, workspaceResolver WorkspaceResolver, portforwarder PortForwarder) *PortForwardOptions {
	return &PortForwardOptions{
		PortForwarder:     portforwarder,
		PortForwardClient: portForwardHelpers,
		WorkspaceResolver: workspaceResolver,
	}
}

func (o *PortForwardOptions) Complete(cmd *cobra.Command, args []string) error {
	workspaceID := args[0]

	workspace, err := o.WorkspaceResolver.GetWorkspaceByID(workspaceID)
	if workspace == nil {
		return fmt.Errorf("workspace with id %s does not exist", workspaceID)
	}
	if err != nil {
		return err
	}

	o.Namespace = workspace.GetNamespaceName()
	o.PodName = workspace.GetPodName()

	o.Address = []string{"localhost"}
	o.Ports = []string{"2222:22"} // TODO override from args

	o.StopChannel = make(chan struct{}, 1)
	o.ReadyChannel = make(chan struct{})

	return nil
}

func (o PortForwardOptions) RunPortforward() error {
	cmd := portforward.NewCmdPortForward(tf, streams) // This command is useful to have around to go to def of kubectl cmd

	pod, err := o.K8sClient.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("unable to forward port because pod is not running. Current status=%v", pod.Status.Phase)
	}

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
	opts.K8sConfig.APIPath = "/api"
	transport, upgrader, err := spdy.RoundTripperFor(opts.K8sConfig)
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
