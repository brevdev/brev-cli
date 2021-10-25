package portforward

import (
	"os"

	"github.com/brevdev/brev-cli/pkg/files"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/portforward"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func RunPortForwardFromCommand(namespace string, podName string, ports []string) {
	args := append([]string{podName}, ports...)
	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.Namespace = &namespace
	keyFilePath := files.GetSSHPrivateKeyFilePath()
	kubeConfigFlags.KeyFile = &keyFilePath
	certFilePath := files.GetCertFilePath()
	kubeConfigFlags.CertFile = &certFilePath
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)

	tf := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	cmd := portforward.NewCmdPortForward(tf, streams)
	cmd.Run(cmd, args)
}
