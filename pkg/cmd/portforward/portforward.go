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

func ConvertNametoSSHName(store PortforwardStore, workspaceNameOrID string) (string, error) {
	org, err := store.GetActiveOrganizationOrDefault()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	workspaces, err2 := store.GetWorkspaceByNameOrID(org.ID, workspaceNameOrID)
	if err2 != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if len(workspaces) == 0 {
		return "", fmt.Errorf("workspace with id/name %s not found", workspaceNameOrID)
	} else if len(workspaces) > 1 {
		return "", fmt.Errorf("duplicate workspace ids: %s", workspaceNameOrID)
	}
	sshName := string(workspaces[0].GetLocalIdentifier())
	return sshName, nil
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
				t.Errprint(err, "Error in workspace name")
				return breverrors.WrapAndTrace(err)
			}

			_, err2 := RunSSHPortForward("-L", portSplit[0], portSplit[1], sshName) // TODO translate from workspace id or name to ssh name
			if err2 != nil {
				t.Errprint(err2, "Failed to port forward")
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

func RunSSHPortForward(forwardType string, localPort string, remotePort string, domainName string) (*os.Process, error) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	portMapping := fmt.Sprintf("%s:127.0.0.1:%s", localPort, remotePort)
	fmt.Printf("ssh -T %s %s %s\n", forwardType, portMapping, domainName)
	cmdSHH := exec.Command("ssh", "-T", forwardType, portMapping, domainName) //nolint:gosec // variables are sanitzed or user specified
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

			workspace, err := getWorkspaceFromNameOrID(args[0], pfStore)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			opts, err = opts.WithWorkspace(*workspace)
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

type WorkspaceResolver struct{}

// NOTE: this function is copy/pasted in many places. If you modify it, modify it elsewhere.
// Reasoning: there wasn't a utils file so I didn't know where to put it
//                + not sure how to pass a generic "store" object
func getWorkspaceFromNameOrID(nameOrID string, sstore PortforwardStore) (*entity.WorkspaceWithMeta, error) {
	// Get Active Org
	org, err := sstore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, breverrors.NewValidationError("no orgs exist")
	}

	// Get Current User
	currentUser, err := sstore.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// Get Workspaces for User
	var workspace *entity.Workspace // this will be the returned workspace
	workspaces, err := sstore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{Name: nameOrID, UserID: currentUser.ID})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	switch len(workspaces) {
	case 0:
		// In this case, check workspace by ID
		wsbyid, othererr := sstore.GetWorkspace(nameOrID) // Note: workspaceName is ID in this case
		if othererr != nil {
			return nil, breverrors.NewValidationError(fmt.Sprintf("no workspaces found with name or id %s", nameOrID))
		}
		if wsbyid != nil {
			workspace = wsbyid
		} else {
			// Can this case happen?
			return nil, breverrors.NewValidationError(fmt.Sprintf("no workspaces found with name or id %s", nameOrID))
		}
	case 1:
		workspace = &workspaces[0]
	default:
		return nil, breverrors.NewValidationError(fmt.Sprintf("multiple workspaces found with name %s\n\nTry running the command by id instead of name:\n\tbrev command <id>", nameOrID))
	}

	if workspace == nil {
		return nil, breverrors.NewValidationError(fmt.Sprintf("no workspaces found with name or id %s", nameOrID))
	}

	// Get WorkspaceMetaData
	workspaceMetaData, err := sstore.GetWorkspaceMetaData(workspace.ID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &entity.WorkspaceWithMeta{WorkspaceMetaData: *workspaceMetaData, Workspace: *workspace}, nil
}
