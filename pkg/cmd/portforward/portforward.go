package portforward

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"

	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	sshLinkLong    = "Port forward your Brev machine's port to your local port"
	sshLinkExample = "brev port-forward <name> -p local_port:remote_port"
)

type PortforwardStore interface {
	completions.CompletionStore
	refresh.RefreshStore
	util.GetWorkspaceByNameOrIDErrStore
	util.MakeWorkspaceWithMetaStore
}

func NewCmdPortForwardSSH(pfStore PortforwardStore, t *terminal.Terminal) *cobra.Command {
	var port string
	var useHost bool
	var org string
	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "port-forward",
		DisableFlagsInUseLine: true,
		Short:                 "Forward a port from instance to local machine",
		Long:                  sshLinkLong,
		Example:               sshLinkExample,
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(pfStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			if port == "" {
				port = startInput(t)
			}
			err := RunPortforward(t, pfStore, args[0], port, useHost, org)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&port, "port", "p", "", "port forward string, local_port:remote_port")
	cmd.Flags().BoolVar(&useHost, "host", false, "Use the -host version of the instance")
	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org)")
	errOrgComp := cmd.RegisterFlagCompletionFunc("org", completions.GetOrgsNameCompletionHandler(pfStore, t))
	if errOrgComp != nil {
		breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(errOrgComp))
		fmt.Print(breverrors.WrapAndTrace(errOrgComp))
	}
	err := cmd.RegisterFlagCompletionFunc("port", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoSpace
	})
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err))
		fmt.Print(breverrors.WrapAndTrace(err))
	}

	return cmd
}

// parsePortString validates and splits a "local:remote" port string.
func parsePortString(portString string) (localPort, remotePort string, err error) {
	if !strings.Contains(portString, ":") {
		return "", "", breverrors.NewValidationError("port format invalid, use local_port:remote_port")
	}
	parts := strings.Split(portString, ":")
	if len(parts) != 2 {
		return "", "", breverrors.NewValidationError("port format invalid, use local_port:remote_port")
	}
	return parts[0], parts[1], nil
}

// isPortAlreadyAllocatedError returns true if the error indicates the port is already open.
func isPortAlreadyAllocatedError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already allocated")
}

func RunPortforward(t *terminal.Terminal, pfStore PortforwardStore, nameOrID string, portString string, useHost bool, orgFlag string) error {
	localPort, remotePort, err := parsePortString(portString)
	if err != nil {
		return err
	}

	res := refresh.RunRefreshAsync(pfStore)

	resolvedOrg, err := util.ResolveOrgFromFlag(pfStore, orgFlag)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	target, err := util.ResolveWorkspaceOrNodeInOrg(pfStore, nameOrID, resolvedOrg.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if target.Node != nil {
		return portForwardExternalNode(t, pfStore, res, target.Node, localPort, remotePort)
	}
	sshName := string(target.Workspace.GetLocalIdentifier())
	if useHost {
		sshName += "-host"
	}

	err = res.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("Port forwarding...\n")
	t.Vprintf("localhost:%s -> %s:%s\n", localPort, sshName, remotePort)
	_, err = RunSSHPortForward("-L", localPort, remotePort, sshName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func portForwardExternalNode(t *terminal.Terminal, pfStore PortforwardStore, res *refresh.RefreshRes, node *nodev1.ExternalNode, localPort, remotePort string) error {
	info, err := util.ResolveExternalNodeSSH(pfStore, node)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Parse the remote port so we can open it via the OpenPort RPC.
	remotePortNum, err := strconv.ParseInt(remotePort, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(fmt.Errorf("invalid remote port %q: %w", remotePort, err))
	}

	// Open the port on the netbird side so it's accessible.
	// This binding persists after the CLI exits — it won't be closed on Ctrl+C.
	t.Vprintf("Opening port %s on node %q...\n", remotePort, node.GetName())
	_, err = util.OpenPort(pfStore, node.GetExternalNodeId(), int32(remotePortNum), nodev1.PortProtocol_PORT_PROTOCOL_TCP)
	if err != nil {
		// Port already allocated is not a real error — it's already open.
		if isPortAlreadyAllocatedError(err) {
			t.Vprintf("Port %s is already open on the remote node.\n", remotePort)
		} else {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		t.Vprintf("Port %s is now bound on the remote node. Note: this binding persists after this command exits.\n", remotePort)
	}

	if err := res.Await(); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// The SSH tunnel forwards local traffic through the SSH connection to the actual port on the box.
	// TODO there isn't support for killing the port forward in either case, and no ClosePort for external node
	alias := info.SSHAlias()
	t.Vprintf("Setting up local forward: localhost:%s -> %s:%s\n", localPort, alias, remotePort)
	_, err = RunSSHPortForward("-L", localPort, remotePort, alias)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func RunSSHPortForward(forwardType string, localPort string, remotePort string, sshName string) (*os.Process, error) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	portMapping := fmt.Sprintf("%s:127.0.0.1:%s", localPort, remotePort)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, breverrors.Wrap(err, "failed to get user home directory")
	}

	keyPath := filepath.Join(homeDir, ".brev", "brev.pem")

	if _, err = os.Stat(keyPath); os.IsNotExist(err) {
		return nil, breverrors.Wrap(err, fmt.Sprintf("SSH key not found at %s. Please ensure your Brev SSH key is properly set up.", keyPath))
	}

	cmdSHH := exec.Command("ssh", "-i", keyPath, "-T", forwardType, portMapping, sshName, "-N") //nolint:gosec //ok
	cmdSHH.Stdin = os.Stdin
	cmdSHH.Stdout = os.Stdout
	cmdSHH.Stderr = os.Stderr

	err = cmdSHH.Start()
	if err != nil {
		return nil, breverrors.Wrap(err, "Failed to start SSH command")
	}

	return cmdSHH.Process, nil
}

func startInput(t *terminal.Terminal) string {
	t.Vprint(t.Yellow("\nPorts flag was omitted, running interactive mode!\n"))
	remoteInput := terminal.PromptGetInput(terminal.PromptContent{
		Label:    "What port on your Brev machine would you like to forward?",
		ErrorMsg: "error",
	})
	localInput := terminal.PromptGetInput(terminal.PromptContent{
		Label:    "What port should it be on your local machine?",
		ErrorMsg: "error",
	})

	port := localInput + ":" + remoteInput

	t.Vprintf("%s", t.Green("\n-p "+port+"\n"))

	return port
}
