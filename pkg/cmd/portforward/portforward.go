package portforward

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/portforward"

	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	sshLinkLong    = "Port forward your Brev machine's port to your local port"
	sshLinkExample = "brev port-forward <ws_name> -p local_port:remote_port"
)

type PortforwardStore interface {
	k8s.K8sStore
	completions.CompletionStore
	refresh.RefreshStore
	util.GetWorkspaceByNameOrIDErrStore
	util.MakeWorkspaceWithMetaStore
}

func NewCmdPortForwardSSH(pfStore PortforwardStore, t *terminal.Terminal) *cobra.Command {
	var port string
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "port-forward",
		DisableFlagsInUseLine: true,
		Short:                 "Enable a local tunnel",
		Long:                  sshLinkLong,
		Example:               sshLinkExample,
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(pfStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			if port == "" {
				port = startInput(t)
			}
			err := runPortforward(pfStore, args[0], port)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&port, "port", "p", "", "port forward flag describe me better")
	err := cmd.RegisterFlagCompletionFunc("port", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoSpace
	})
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err))
		fmt.Print(breverrors.WrapAndTrace(err))
	}

	return cmd
}

func runPortforward(pfStore PortforwardStore, nameOrID string, portString string) error {
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

	sshName, err := ConvertNametoSSHName(pfStore, nameOrID)
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

func ConvertNametoSSHName(store PortforwardStore, workspaceNameOrID string) (string, error) {
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(store, workspaceNameOrID)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	sshName := string(workspace.GetLocalIdentifier())
	return sshName, nil
}

func RunSSHPortForward(forwardType string, localPort string, remotePort string, sshName string) (*os.Process, error) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	portMapping := fmt.Sprintf("%s:127.0.0.1:%s", localPort, remotePort)
	cmdSHH := exec.Command("ssh", "-T", forwardType, portMapping, sshName, "-N") //nolint:gosec // variables are sanitzed or user specified
	cmdSHH.Stdin = os.Stdin
	fmt.Println("portforwarding...")
	fmt.Printf("localhost:%s -> %s:%s\n", localPort, sshName, remotePort)
	out, err := cmdSHH.CombinedOutput()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err, string(out))
	}

	return cmdSHH.Process, nil
}

func NewCmdPortForward(pfStore PortforwardStore, t *terminal.Terminal) *cobra.Command {
	var port string
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "port-forward",
		DisableFlagsInUseLine: true,
		Short:                 "Enable a local ssh link tunnel",
		Long:                  sshLinkLong,
		Example:               sshLinkExample,
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(pfStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			if port == "" {
				port = startInput(t)
			}

			k8sClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(pfStore)
			if err != nil {
				switch err.(type) {
				case *url.Error:
					return breverrors.WrapAndTrace(err, "check your internet connection")
				default:
					return breverrors.WrapAndTrace(err)
				}
			}
			pf := portforward.NewDefaultPortForwarder()

			opts := portforward.NewPortForwardOptions(
				k8sClientMapper,
				pf,
			)

			workspace, err := util.GetUserWorkspaceByNameOrIDErr(pfStore, args[0])
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			workspaceWithMeta, err := util.MakeWorkspaceWithMeta(pfStore, workspace)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			opts, err = opts.WithWorkspace(workspaceWithMeta)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			opts.WithPort(port)

			err = opts.RunPortforward()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&port, "port", "p", "", "port forward flag describe me better")
	err := cmd.RegisterFlagCompletionFunc("port", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoSpace
	})
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err))
		fmt.Print(breverrors.WrapAndTrace(err))
	}

	return cmd
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

	t.Vprintf(t.Green("\n-p " + port + "\n"))

	return port
}
