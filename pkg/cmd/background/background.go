package background

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

type BackgroundStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	ModifyWorkspace(workspaceID string, options *store.ModifyWorkspaceRequest) (*entity.Workspace, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentWorkspaceID() (string, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
}

func DisableAutoStop(s BackgroundStore, workspaceID string) error {
	isStoppable := false
	_, err := s.ModifyWorkspace(workspaceID, &store.ModifyWorkspaceRequest{
		WorkspaceClassID: workspaceID,
		IsStoppable:      &isStoppable,
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func EnableAutoStop(s BackgroundStore, workspaceID string) error {
	isStoppable := true
	_, err := s.ModifyWorkspace(workspaceID, &store.ModifyWorkspaceRequest{
		WorkspaceClassID: workspaceID,
		IsStoppable:      &isStoppable,
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func NewCmdBackground(t *terminal.Terminal, s BackgroundStore) *cobra.Command {
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
			progressFlag, err := cmd.Flags().GetBool("progress")
			if err != nil {
				log.Fatal(err)
			}
			if progressFlag {
				checkProgress()
				return nil
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

			wsID, err := s.GetCurrentWorkspaceID()
			if err == nil {
				// Disable auto stop
				err = DisableAutoStop(s, wsID)
				if err != nil {
					log.Fatal(err)
				}
			}

			// Run the command in the background using nohup
			c := exec.Command("nohup", "bash", "-c", command+">"+logsDir+"/log.txt 2>&1 &") // #nosec G204
			err = c.Start()
			if err != nil {
				log.Fatal(err)
			}

			// Write logs
			logFile, err := os.OpenFile(logsDir+"/log.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666) //nolint: gosec // TODO
			if err != nil {
				log.Fatal(err)
			}

			defer logFile.Close() //nolint: gosec,errcheck // TODO

			logFile.WriteString(time.Now().Format("2006-01-02 15:04:05") + ": Command \"" + command + "\" was run in the background.\n") //nolint: errcheck,gosec // TODO

			// Write process details to data file
			processesFile, err := os.OpenFile(logsDir+"/processes.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666) //nolint: gosec // TODO
			if err != nil {
				log.Fatal(err)
			}
			defer processesFile.Close() //nolint: errcheck,gosec // TODO
			_, _ = processesFile.WriteString(fmt.Sprintf("%d,%s,%s\n", c.Process.Pid, time.Now().Format("2006-01-02 15:04:05"), command))

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
	cmd.Flags().Bool("progress", false, "Show progress of the background commands")

	return cmd
}

type Process struct {
	ID        int    `json:"id"`
	Command   string `json:"command"`
	LogsDir   string `json:"logsDir"`
	StartTime string `json:"startTime"`
}

type Processes struct {
	Processes []Process `json:"processes"`
}

func checkProgress() {
	filePath := fmt.Sprintf("%s/brev-background-logs/processes.txt", os.Getenv("HOME"))
	file, err := os.Open(filePath) //nolint: gosec // TODO
	if err != nil {
		panic(err)
	}
	defer file.Close() //nolint: errcheck,gosec // TODO

	scanner := bufio.NewScanner(file)

	fmt.Println("Running processes:")
	for scanner.Scan() {
		processLine := scanner.Text()
		processData := strings.Split(processLine, ",")

		startTime, err := time.Parse("2006-01-02 15:04:05", processData[1])
		if err != nil {
			panic(err)
		}

		// Check if the process is still running by its PID
		processID := processData[0]
		cmd := fmt.Sprintf("ps -p %s -o comm=", processID)
		output, err := exec.Command("bash", "-c", cmd).Output() //nolint: gosec // TODO

		if err != nil || strings.TrimSpace(string(output)) == "" {
			// Process is not running, show a checkmark
			fmt.Printf("ID: %s \u2713\n", processID)
		} else {
			// Process is still running, display its info
			fmt.Printf("ID: %s\n", processID)
			fmt.Printf("Command: %s\n", processData[2])
			fmt.Printf("Start time: %s\n", startTime.Format("2006-01-02 15:04:05"))
		}

		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}
}
