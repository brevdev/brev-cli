package portforward

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

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
	sshLinkExample = "brev port-forward <ws_name> -p local_port:remote_port"
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
			err := RunPortforward(pfStore, args[0], port, useHost)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&port, "port", "p", "", "port forward flag describe me better")
	cmd.Flags().BoolVar(&useHost, "host", false, "Use the -host version of the instance")
	err := cmd.RegisterFlagCompletionFunc("port", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoSpace
	})
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err))
		fmt.Print(breverrors.WrapAndTrace(err))
	}

	return cmd
}

func RunPortforward(pfStore PortforwardStore, nameOrID string, portString string, useHost bool) error {
	var portSplit []string
	if strings.Contains(portString, ":") {
		portSplit = strings.Split(portString, ":")
		if len(portSplit) != 2 {
			return breverrors.NewValidationError("port format invalid, use local_port:remote_port")
		}
	} else {
		return breverrors.NewValidationError("port format invalid, use local_port:remote_port")
	}

	res := refresh.RunRefreshAsync(pfStore)

	sshName, err := ConvertNametoSSHName(pfStore, nameOrID, useHost)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = res.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = RunSSHPortForward("-L", portSplit[0], portSplit[1], sshName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func ConvertNametoSSHName(store PortforwardStore, workspaceNameOrID string, useHost bool) (string, error) {
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(store, workspaceNameOrID)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	sshName := string(workspace.GetLocalIdentifier())
	if useHost {
		sshName += "-host"
	}
	return sshName, nil
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

	fmt.Println("Port forwarding...")
	fmt.Printf("localhost:%s -> %s:%s\n", localPort, sshName, remotePort)

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
