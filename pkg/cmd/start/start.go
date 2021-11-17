// Package start is for starting Brev workspaces
package start

import (
	"fmt"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var (
	startLong    = "Start a Brev machine that's in a paused or off state or create one from a url"
	startExample = `
  brev start <existing_ws_name>
  brev start <git url>
  brev start <git url> --org myFancyOrg
	`
)

func NewCmdStart(t *terminal.Terminal) *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "start",
		DisableFlagsInUseLine: true,
		Short:                 "Start a workspace if it's stopped, or create one from url",
		Long:                  startLong,
		Example:               startExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             brevapi.GetCachedWorkspaceNames(),
		Run: func(cmd *cobra.Command, args []string) {
			isURL := false
			if strings.Contains(args[0], "https://") || strings.Contains(args[0], "git@") {
				isURL = true
			}

			if !isURL {
				err := startWorkspace(args[0], t)
				if err != nil {
					t.Vprint(t.Red(err.Error()))
				}
				return

			} else {

				// CREATE A WORKSPACE
				err := clone(t, args[0], org)
				if err != nil {
					t.Vprint(t.Red(err.Error()))
				}
			}
		},
	}
	cmd.Flags().StringVarP(&org, "org", "o", "", "organization (will override active org if creating a workspace)")
	err := cmd.RegisterFlagCompletionFunc("org", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return brevapi.GetOrgNames(), cobra.ShellCompDirectiveNoSpace
	})
	if err != nil {
		t.Errprint(err, "cli err")
	}

	return cmd
}

func startWorkspace(workspaceName string, t *terminal.Terminal) error {
	client, err := brevapi.NewCommandClient()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspace, err := brevapi.GetWorkspaceFromName(workspaceName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	startedWorkspace, err := client.StartWorkspace(workspace.ID)
	if err != nil {
		fmt.Println(err)
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf(t.Yellow("\nWorkspace %s is starting. \nNote: this can take about a minute. Run 'brev ls' to check status\n\n", startedWorkspace.Name))

	w, err := pollUntil(t, workspace.ID, "RUNNING")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour workspace is ready!"))

	t.Vprintf(t.Green("\n\nTo connect to your machine, make sure to Brev up:") +
		t.Yellow("\n\t$ brev up\n"))

	t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.Name))

	return nil
}

// "https://github.com/brevdev/microservices-demo.git
// "https://github.com/brevdev/microservices-demo.git"
// "git@github.com:brevdev/microservices-demo.git"
func clone(t *terminal.Terminal, url string, orgflag string) error {
	formattedURL := brevapi.ValidateGitURL(t, url)

	var orgID string
	if orgflag == "" {
		activeorg, err := brevapi.GetActiveOrgContext(files.AppFs)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		orgID = activeorg.ID
	} else {
		org, err := brevapi.GetOrgFromName(orgflag)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		orgID = org.ID
	}

	err := createWorkspace(t, formattedURL, orgID)
	if err != nil {
		t.Vprint(t.Red(err.Error()))
	}
	return nil
}

func createWorkspace(t *terminal.Terminal, newworkspace brevapi.NewWorkspace, orgID string) error {
	c, err := brevapi.NewClient()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprint("\nWorkspace is starting. " + t.Yellow("This can take up to 2 minutes the first time.\n"))
	w, err := c.CreateWorkspace(orgID, newworkspace.Name, newworkspace.GitRepo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = pollUntil(t, w.ID, "RUNNING")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("\nYour workspace is ready!"))
	t.Vprintf(t.Green("\nSSH into your machine:\n\tssh %s\n", w.Name))
	// t.Vprintf("\nor use in browser: \n\thttps://%s", w.DNS)

	return nil
}

func pollUntil(t *terminal.Terminal, wsid string, state string) (*brevapi.Workspace, error) {
	c, err := brevapi.NewClient()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	s := t.NewSpinner()
	isReady := false
	var ws *brevapi.Workspace
	for !isReady {
		time.Sleep(5 * time.Second)
		s.Start()
		ws, err = c.GetWorkspace(wsid)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		s.Suffix = "  workspace is " + strings.ToLower(ws.Status)
		if ws.Status == state {
			s.Suffix = "Workspace is ready!"
			s.Stop()
			isReady = true
		}
	}
	return ws, nil
}
