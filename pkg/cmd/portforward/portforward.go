package portforward

import (
	"fmt"
	"net/url"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"

	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"
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
	GetWorkspaceMetaData(workspaceID string) (*brevapi.WorkspaceMetaData, error)
	GetWorkspace(workspaceID string) (*brevapi.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]brevapi.Workspace, error)
	GetCurrentUser() (*brevapi.User, error)
}

func NewCmdPortForward(pfStore PortforwardStore, t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "port-forward",
		DisableFlagsInUseLine: true,
		Short:                 "Enable a local ssh link tunnel",
		Long:                  sshLinkLong,
		Example:               sshLinkExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(pfStore, t),
		Run: func(cmd *cobra.Command, args []string) {

			if Port=="" {
				startInput(t)
			}

			k8sClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(pfStore)
			if err != nil {
				switch err.(type) {
				case *url.Error:
					t.Errprint(err, "\n\ncheck your internet connection")
					return

				default:
					t.Errprint(err, "")
					return
				}
			}
			pf := portforward.NewDefaultPortForwarder()

			opts := portforward.NewPortForwardOptions(
				k8sClientMapper,
				pf,
			)

			workspace, err := GetWorkspaceByIDOrName(args[0], pfStore)
			if err != nil {
				t.Errprint(err, "")
				return
			}

			opts, err = opts.WithWorkspace(*workspace)
			if err != nil {
				t.Errprint(err, "")
				return
			}

			opts.WithPort(Port)

			err = opts.RunPortforward()
			if err != nil {
				t.Errprint(err, "")
				return
			}
		},
	}
	cmd.Flags().StringVarP(&Port, "port", "p", "", "port forward flag describe me better")
	err := cmd.RegisterFlagCompletionFunc("port", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoSpace
	})
	if err != nil {
		t.Errprint(err, "cli err")
	}

	return cmd
}

func startInput(t *terminal.Terminal) {
	t.Vprint(t.Yellow("\nPorts flag was omitted, running interactive mode!\n"))
	remoteInput := brevapi.PromptGetInput(brevapi.PromptContent{
		Label:    "What port on your Brev machine would you like to forward?",
		ErrorMsg: "error",
	})
	localInput := brevapi.PromptGetInput(brevapi.PromptContent{
		Label:    "What port should it be on your local machine?",
		ErrorMsg: "error",
	})

	Port = localInput + ":" + remoteInput

	t.Vprintf(t.Green("\n-p " + Port + "\n"))

	t.Printf("\nStarting ssh link...\n")
}

type WorkspaceResolver struct{}

func GetWorkspaceByIDOrName(workspaceIDOrName string, pfStore PortforwardStore) (*brevapi.WorkspaceWithMeta, error) {
	currentUser, err := pfStore.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	workspaces, err := pfStore.GetAllWorkspaces(&store.GetWorkspacesOptions{Name: workspaceIDOrName, UserID: currentUser.ID})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var workspace *brevapi.Workspace
	if len(workspaces) > 1 { //nolint:gocritic // ok to use if else here
		return nil, fmt.Errorf("multiple workspaces with found with name %s", workspaceIDOrName)
	} else if len(workspaces) == 1 {
		workspace = &workspaces[0]
	} else {
		workspace, err = pfStore.GetWorkspace(workspaceIDOrName)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}

	workspaceMetaData, err := pfStore.GetWorkspaceMetaData(workspace.ID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &brevapi.WorkspaceWithMeta{WorkspaceMetaData: *workspaceMetaData, Workspace: *workspace}, nil
}
