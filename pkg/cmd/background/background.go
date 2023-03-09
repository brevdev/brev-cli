package background

import (
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

var (
	createLong    = "Create a new Brev machine"
	createExample = `
  brev create <name>
	`
	// instanceTypes = []string{"p4d.24xlarge", "p3.2xlarge", "p3.8xlarge", "p3.16xlarge", "p3dn.24xlarge", "p2.xlarge", "p2.8xlarge", "p2.16xlarge", "g5.xlarge", "g5.2xlarge", "g5.4xlarge", "g5.8xlarge", "g5.16xlarge", "g5.12xlarge", "g5.24xlarge", "g5.48xlarge", "g5g.xlarge", "g5g.2xlarge", "g5g.4xlarge", "g5g.8xlarge", "g5g.16xlarge", "g5g.metal", "g4dn.xlarge", "g4dn.2xlarge", "g4dn.4xlarge", "g4dn.8xlarge", "g4dn.16xlarge", "g4dn.12xlarge", "g4dn.metal", "g4ad.xlarge", "g4ad.2xlarge", "g4ad.4xlarge", "g4ad.8xlarge", "g4ad.16xlarge", "g3s.xlarge", "g3.4xlarge", "g3.8xlarge", "g3.16xlarge"}
)

type BackgroundStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentWorkspaceID() (string, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
}

func NewCmdBackground(t *terminal.Terminal, backgroundStore BackgroundStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "background [flags] [command]",
		Aliases:               []string{"bg"},
		DisableFlagsInUseLine: true,
		Short:                 "Run a command in the background with optional 'brev stop self' at the end",
		Long:                  "This command will run a specified command in the background using nohup and write logs to $HOME/brev-background-logs.",
		Example:               "background ./myscript.sh --stop",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse the flags
			stopFlag, err := cmd.Flags().GetBool("stop")
			if err != nil {
				log.Fatal(err)
			}

			// Join the args into a command string
			command := ""
			if len(args) > 0 {
				command = args[0]
			}
			for i := 1; i < len(args); i++ {
				command += " " + args[i]
			}

			// Create logs directory if it doesn't exist
			logsDir := os.Getenv("HOME") + "/brev-background-logs"
			err = os.MkdirAll(logsDir, os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}

			// Run the command in the background using nohup
			c := exec.Command("nohup", "bash", "-c", command+">"+logsDir+"/log.txt 2>&1 &")
			err = c.Start()
			if err != nil {
				log.Fatal(err)
			}

			// Write logs
			logFile, err := os.OpenFile(logsDir+"/log.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
			if err != nil {
				log.Fatal(err)
			}
			defer logFile.Close()
			logFile.WriteString(time.Now().Format("2006-01-02 15:04:05") + ": Command \"" + command + "\" was run in the background.\n")

			if stopFlag {
				// If --stop flag is set, run "brev stop self" at the end
				defer func() {
					stopCmd := exec.Command("brev", "stop", "self")
					err = stopCmd.Run()
					if err != nil {
						log.Fatal(err)
					}
				}()
			}

			t.Vprintf("Command \"%s\" has been run in the background. Check %s for logs.\n", command, logsDir)
			return nil
		},
	}
	cmd.Flags().Bool("stop", false, "Stop the workspace after the command is finished")
	return cmd
}
