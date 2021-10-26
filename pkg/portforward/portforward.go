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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolsportforward "k8s.io/client-go/tools/portforward"
)

type PortForwardOptions struct {
	PortForwarder PortForwarder
	K8sClient     *kubernetes.Clientset
	K8sConfig     *rest.Config

	Namespace string
	PodName   string

	Address      []string
	Ports        []string
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

func NewPortForwardOptions(k8sClient *kubernetes.Clientset, portforwarder PortForwarder) *PortForwardOptions {
	return &PortForwardOptions{
		PortForwarder: portforwarder,
		K8sClient:     k8sClient,
	}
}

func (o *PortForwardOptions) Complete(cmd *cobra.Command, args []string) error {
	o.Namespace = "w8s-pftest-cc39-alecfong1hq"                // TODO get ns
	o.PodName = "w8s-pftest-cc39-alecfong1hq-7649c85f94-gjq6l" // TODO get podName

	o.Address = []string{"localhost"}
	o.Ports = []string{"2222:22"} // TODO override from args

	o.StopChannel = make(chan struct{}, 1)
	o.ReadyChannel = make(chan struct{})

	return nil
}

func (o PortForwardOptions) RunPortforward() error {
	// cmd := portforward.NewCmdPortForward(tf, streams)

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

	req := o.K8sClient.RESTClient().Post().
		Resource("pods").
		Namespace(o.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	return o.PortForwarder.ForwardPorts("POST", req.URL(), o)
}

// how do we specify port
// how do we do auth
// vi client how do we specify namespace & pod

type PortForwarder interface {
	ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error
}

type DefaultPortForwarder struct {
	genericclioptions.IOStreams
}

func (f *DefaultPortForwarder) ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error {
	transport, upgrader, err := spdy.RoundTripperFor(opts.K8sConfig)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)
	fw, err := toolsportforward.NewOnAddresses(dialer, opts.Address, opts.Ports, opts.StopChannel, opts.ReadyChannel, f.Out, f.ErrOut)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}
