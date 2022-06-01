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
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/brevdev/brev-cli/pkg/store"

	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	Port           string
	sshLinkLong    = "Port forward your Brev machine's port to your local port"
	sshLinkExample = "brev port-forward <ws_name> -p local_port:remote_port"
)

type PortforwardStore interface {
	k8s.K8sStore
	completions.CompletionStore
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetCurrentUser() (*entity.User, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error)
}

func NewCmdPortForwardSSH(pfStore PortforwardStore, t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "port-forwardssh",
		DisableFlagsInUseLine: true,
		Short:                 "Enable a local tunnel",
		Long:                  sshLinkLong,
		Example:               sshLinkExample,
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(pfStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			if Port == "" {
				startInput(t)
			}
			var portSplit []string
			if strings.Contains(Port, ":") {
				portSplit = strings.Split(Port, ":")
				if len(portSplit) != 2 {
					return breverrors.NewValidationError("port format invalid, use local_port:remote_port")
				}
			} else {
				return breverrors.NewValidationError("port format invalid, use local_port:remote_port")
			}

			sshName, err := ConvertNametoSSHName(pfStore, args[0])
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			_, err = RunSSHPortForward("-L", portSplit[0], portSplit[1], sshName)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&Port, "port", "p", "", "port forward flag describe me better")
	err := cmd.RegisterFlagCompletionFunc("port", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoSpace
	})
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(err)
		t.Errprint(err, "cli err")
	}

	return cmd
}

func ConvertNametoSSHName(store PortforwardStore, workspaceNameOrID string) (string, error) {
	workspace, err := util.GetWorkspaceByNameOrIDErr(store, workspaceNameOrID)
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
	fmt.Printf("ssh -T %s %s %s\n", forwardType, portMapping, sshName)
	cmdSHH := exec.Command("ssh", "-T", forwardType, portMapping, sshName) //nolint:gosec // variables are sanitzed or user specified
	cmdSHH.Stdin = os.Stdin
	cmdSHH.Stderr = os.Stderr        // TODO remove
	cmdSHH.Stdout = os.Stdout        // TODO remove
	fmt.Println("portforwarding...") // TODO better messaging
	err := cmdSHH.Run()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return cmdSHH.Process, nil
}

func NewCmdPortForward(pfStore PortforwardStore, t *terminal.Terminal) *cobra.Command {
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
			if Port == "" {
				startInput(t)
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

			workspace, err := util.GetWorkspaceByNameOrIDErr(pfStore, args[0])
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

			opts.WithPort(Port)

			err = opts.RunPortforward()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&Port, "port", "p", "", "port forward flag describe me better")
	err := cmd.RegisterFlagCompletionFunc("port", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoSpace
	})
	if err != nil {
		breverrors.GetDefaultErrorReporter().ReportError(err)
		t.Errprint(err, "cli err")
	}

	return cmd
}

func startInput(t *terminal.Terminal) {
	t.Vprint(t.Yellow("\nPorts flag was omitted, running interactive mode!\n"))
	remoteInput := terminal.PromptGetInput(terminal.PromptContent{
		Label:    "What port on your Brev machine would you like to forward?",
		ErrorMsg: "error",
	})
	localInput := terminal.PromptGetInput(terminal.PromptContent{
		Label:    "What port should it be on your local machine?",
		ErrorMsg: "error",
	})

	Port = localInput + ":" + remoteInput

	t.Vprintf(t.Green("\n-p " + Port + "\n"))

	t.Printf("\nStarting ssh link...\n")
}
