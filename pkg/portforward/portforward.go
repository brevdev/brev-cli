package portforward

import (
	"net/url"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type PortForwardOptions struct {
	PortForwarder PortForwarder

	Address      []string
	Ports        []string
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

type PortForwarder interface {
	ForwardPorts(method string, url *url.URL, opts PortForwardOptions) error
}

// TODO with stopchannel

// cmd := portforward.NewCmdPortForward(tf, streams) // This command is useful to have around to go to def of kubectl cmd

type DefaultPortForwarder struct {
	genericclioptions.IOStreams
}
