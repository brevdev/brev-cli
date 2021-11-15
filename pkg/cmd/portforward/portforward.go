// Package link is for the ssh command
package portforward

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/manifoldco/promptui"

	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/portforward"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

type promptContent struct {
	errorMsg string
	label    string
}

var (
	Port           string
	sshLinkLong    = "Port forward your Brev machine's port to your local port"
	sshLinkExample = "brev link <ws_name> -p local_port:remote_port"
)

func promptGetInput(pc promptContent) string {
	validate := func(input string) error {
		if len(input) <= 0 {
			return errors.New(pc.errorMsg)
		}
		return nil
	}

	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | red }} ",
		Success: "{{ . | bold }} ",
	}

	prompt := promptui.Prompt{
		Label:     pc.label,
		Templates: templates,
		Validate:  validate,
	}

	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	return result
}

func NewCmdPortForward(t *terminal.Terminal) *cobra.Command {
	// link [resource id] -p 2222
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "port-forward",
		DisableFlagsInUseLine: true,
		Short:                 "Enable a local ssh link tunnel",
		Long:                  sshLinkLong,
		Example:               sshLinkExample,
		Args:                  cobra.ExactArgs(1),
		ValidArgs:             brevapi.GetWorkspaceNames(),
		Run: func(cmd *cobra.Command, args []string) {
			t.Vprintf(Port + "\n\n\n")
			t.Vprint(t.Yellow("\nPorts flag was omitted, running interactive mode!"))
			remoteInput := promptGetInput(promptContent{
				label:    "What port on your Brev machine would you like to forward?",
				errorMsg: "error",
			})
			localInput := promptGetInput(promptContent{
				label:    "What port should it be on your local machine?",
				errorMsg: "error",
			})

			Port = localInput + ":" + remoteInput

			t.Vprintf(t.Green("\n-p " + Port + "\n"))

			t.Printf("\nStarting ssh link...\n")
			client, err := brevapi.NewCommandClient() // to inject
			if err != nil {
				t.Errprint(err, "")
				return
			}
			k8sClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(client) // to resolve
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
			err = files.WriteSSHPrivateKey(files.AppFs, k8sClientMapper.GetPrivateKey())
			if err != nil {
				t.Errprint(err, "")
				return
			}
			sshPrivateKeyFilePath := files.GetSSHPrivateKeyFilePath()
			if Port == "" {
				Port = "2222:22"
			}

			workspace, err := GetWorkspaceByIDOrName(args[0], WorkspaceResolver{})
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

			t.Printf("SSH Private Key: %s\n", sshPrivateKeyFilePath)
			t.Printf(t.Green("\n\t1. Add SSH Key:\n"))
			t.Printf(t.Yellow("\t\tssh-add %s\n", sshPrivateKeyFilePath))
			t.Printf(t.Green("\t2. Connect to workspace:\n"))
			localPort := strings.Split(Port, ":")[0]
			t.Printf(t.Yellow("\t\tssh -p %s brev@0.0.0.0\n\n", localPort))
			err = opts.RunPortforward()
			if err != nil {
				t.Errprint(err, "")
				return
			}
		},
	}
	cmd.Flags().StringVarP(&Port, "port", "p", "", "port forward flag describe me better")
	cmd.RegisterFlagCompletionFunc("port", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoSpace
	})

	return cmd
}

type WorkspaceResolver struct{}

func GetWorkspaceByIDOrName(workspaceIDOrName string, workspaceResolver WorkspaceResolver) (*brevapi.WorkspaceWithMeta, error) {
	workspace, err := workspaceResolver.GetWorkspaceByID(workspaceIDOrName)
	if err != nil {
		wsByName, err2 := workspaceResolver.GetWorkspaceByName(workspaceIDOrName)
		if err2 != nil {
			return nil, err2
		} else {
			workspace = wsByName
		}
	}
	if workspace == nil {
		return nil, fmt.Errorf("workspace does not exist [identifier=%s]", workspaceIDOrName)
	}
	return workspace, nil
}

func (d WorkspaceResolver) GetWorkspaceByID(id string) (*brevapi.WorkspaceWithMeta, error) {
	c, err := brevapi.NewCommandClient()
	if err != nil {
		return nil, err
	}
	w, err := c.GetWorkspace(id)
	if err != nil {
		return nil, err
	}
	wmeta, err := c.GetWorkspaceMetaData(id)
	if err != nil {
		return nil, err
	}

	return &brevapi.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: *w}, nil
}

// This function will be long and messy, it's entirely built to check random error cases
// func GetWorkspaceByName(name string) (*brevapi.AllWorkspaceData, error) {
func (d WorkspaceResolver) GetWorkspaceByName(name string) (*brevapi.WorkspaceWithMeta, error) {
	c, err := brevapi.NewCommandClient()
	if err != nil {
		return nil, err
	}

	// Check ActiveOrg's workspaces before checking every orgs workspaces as fallback
	activeorg, err := brevapi.GetActiveOrgContext(files.AppFs)
	if err != nil {
		// TODO: we sould just check all possible workspaces here
		return nil, errors.New("Please set your active org or link to a workspace by it's ID")
	} else {
		workspaces, err := c.GetMyWorkspaces(activeorg.ID)
		if err != nil {
			return nil, err
		}
		for _, w := range workspaces {
			if w.Name == name {
				wmeta, err := c.GetWorkspaceMetaData(w.ID)
				if err != nil {
					return nil, err
				}
				return &brevapi.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: w}, nil
			}
		}
		// if there wasn't a workspace in the org, check all the orgs
	}

	orgs, err := c.GetOrgs()
	if err != nil {
		return nil, err
	}

	for _, o := range orgs {
		workspaces, err := c.GetWorkspaces(o.ID)
		if err != nil {
			return nil, err
		}

		for _, w := range workspaces {
			if w.Name == name {
				// Assemble full object
				wmeta, err := c.GetWorkspaceMetaData(w.ID)
				if err != nil {
					return nil, err
				}
				return &brevapi.WorkspaceWithMeta{WorkspaceMetaData: *wmeta, Workspace: w}, nil
			}
		}
	}

	return nil, fmt.Errorf("workspace does not exist [name=%s]", name)
}
