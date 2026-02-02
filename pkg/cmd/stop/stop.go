// Package stop is for stopping Brev workspaces
package stop

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
)

var (
	stopLong    = "Stop a Brev machine that's in a running state"
	stopExample = "brev stop <ws_name>...\nbrev stop --all\necho instance-name | brev stop"
)

type StopStore interface {
	completions.CompletionStore
	util.GetWorkspaceByNameOrIDErrStore
	StopWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentUser() (*entity.User, error)
	IsWorkspace() (bool, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetCurrentWorkspaceID() (string, error)
}

func NewCmdStop(t *terminal.Terminal, loginStopStore StopStore, noLoginStopStore StopStore) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"provider-dependent": ""},
		Use:                   "stop",
		DisableFlagsInUseLine: true,
		Short:                 "Stop an instance if it's running",
		Long:                  stopLong,
		Example:               stopExample,
		// Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs()),
		ValidArgsFunction: completions.GetAllWorkspaceNameCompletionHandler(noLoginStopStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			piped := isStdoutPiped()
			if all {
				return stopAllWorkspaces(t, loginStopStore, piped)
			} else {
				names, err := getInstanceNames(args)
				if err != nil {
					return err
				}
				var allErr error
				var stoppedNames []string
				for _, name := range names {
					err := stopWorkspace(name, t, loginStopStore, piped)
					if err != nil {
						allErr = multierror.Append(allErr, err)
					} else {
						stoppedNames = append(stoppedNames, name)
					}
				}
				// Output names for piping to next command
				if piped {
					for _, name := range stoppedNames {
						fmt.Println(name)
					}
				}
				if allErr != nil {
					return breverrors.WrapAndTrace(allErr)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "stop all instances")

	return cmd
}

func stopAllWorkspaces(t *terminal.Terminal, stopStore StopStore, piped bool) error {
	user, err := stopStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	org, err := stopStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaces, err := stopStore.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{UserID: user.ID})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !piped {
		t.Vprintf("Turning off all of your instances")
	}
	var stoppedNames []string
	for _, v := range workspaces {
		if v.Status == entity.Running {
			_, err = stopStore.StopWorkspace(v.ID)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			} else {
				stoppedNames = append(stoppedNames, v.Name)
				if !piped {
					t.Vprintf("%s", t.Green("\n%s stopped ✓", v.Name))
				}
			}
		}
	}
	// Output names for piping to next command
	if piped {
		for _, name := range stoppedNames {
			fmt.Println(name)
		}
	}
	return nil
}

func stopWorkspace(workspaceName string, t *terminal.Terminal, stopStore StopStore, piped bool) error {
	user, err := stopStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var workspaceID string

	if workspaceName == "self" {
		wsID, err2 := stopStore.GetCurrentWorkspaceID()
		if err2 != nil {
			if !piped {
				t.Vprintf("\n Error: %s", t.Red(err2.Error()))
			}
			return breverrors.WrapAndTrace(err2)
		}
		workspaceID = wsID
	} else {
		workspace, err3 := util.GetUserWorkspaceByNameOrIDErr(stopStore, workspaceName)
		if err3 != nil {
			if !strings.Contains(err3.Error(), "not found") {
				return breverrors.WrapAndTrace(err3)
			} else {
				if user.GlobalUserType == entity.Admin {
					if !piped {
						fmt.Println("admin trying to stop any instance")
					}
					workspace, err = util.GetAnyWorkspaceByIDOrNameInActiveOrgErr(stopStore, workspaceName)
					if err != nil {
						return breverrors.WrapAndTrace(err)
					}
				} else {
					return breverrors.WrapAndTrace(err)
				}
			}
		}
		workspaceID = workspace.ID
	}

	_, err = stopStore.StopWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	} else if !piped {
		if workspaceName == "self" {
			t.Vprintf("%s", t.Green("Stopping this instance\n")+
				"Note: this can take a few seconds. Run 'brev ls' to check status\n")
		} else {
			t.Vprintf("%s", t.Green("Stopping instance "+workspaceName+".\n")+
				"Note: this can take a few seconds. Run 'brev ls' to check status\n")
		}
	}

	return nil
}

// isStdoutPiped returns true if stdout is being piped to another command
func isStdoutPiped() bool {
	stat, _ := os.Stdout.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// getInstanceNames gets instance names from args or stdin (supports piping)
func getInstanceNames(args []string) ([]string, error) {
	var names []string

	// Add names from args
	names = append(names, args...)

	// Check if stdin is piped
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Stdin is piped, read instance names (one per line)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			name := strings.TrimSpace(scanner.Text())
			if name != "" {
				names = append(names, name)
			}
		}
	}

	if len(names) == 0 {
		return nil, breverrors.NewValidationError("instance name required: provide as argument or pipe from another command")
	}

	return names, nil
}
