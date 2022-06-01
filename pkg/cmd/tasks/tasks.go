package tasks

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/server"
	"github.com/brevdev/brev-cli/pkg/ssh"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/vpn"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type TaskMap map[string]tasks.Task

var all bool // used for run command

type TaskStore interface {
	CopyBin(targetBin string) error
	WriteString(path, data string) error
	RegisterNode(publicKey string) error
	GetOrCreateFile(path string) (afero.File, error)
	GetNetworkAuthKey() (*store.GetAuthKeyResponse, error)
	GetCurrentWorkspaceID() (string, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentUser() (*entity.User, error)
	vpn.ServiceMeshStore
	server.RPCServerTaskStore
	ssh.ConfigUpaterFactoryStore
}

func NewCmdTasks(t *terminal.Terminal, store TaskStore) *cobra.Command {
	taskMap := getTaskMap(store)
	cmd := &cobra.Command{
		Use:                   "tasks",
		DisableFlagsInUseLine: true,
		Short:                 "run background daemons for brev",
		Long:                  "run background daemons for brev",
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(0)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("invalid command")
		},
	}

	configure := NewCmdConfigure(t, store)
	cmd.AddCommand(configure)
	run := NewCmdRun(t, store, taskMap)
	run.PersistentFlags().BoolVarP(&all, "all", "a", false, "specifies all tasks")
	cmd.AddCommand(run)
	return cmd
}

func NewCmdConfigure(_ *terminal.Terminal, store TaskStore) *cobra.Command {
	taskMap := getTaskMap(store)
	cmd := &cobra.Command{
		Use:   "configure [task to configure]",
		Short: "configure system startup daemon for task",
		Long:  "configure system startup daemon for task",
		RunE: func(cmd *cobra.Command, args []string) error {
			// todo if --user flag is not provided and if not run as root, raise
			// an error
			fmt.Println("configuring...")
			var allError error
			for k, value := range taskMap {
				fmt.Printf("configuring %s\n", k)
				err := value.Configure()
				if err != nil {
					fmt.Println(k)
					allError = multierror.Append(allError, err)
				}
			}
			fmt.Println("done configuring")
			if allError != nil {
				return breverrors.WrapAndTrace(allError)
			}
			return nil
		},
	}
	return cmd
}

func NewCmdRun(_ *terminal.Terminal, _ TaskStore, taskMap TaskMap) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [task to configure]",
		Short: "run a task",
		Long:  "run a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				var allError error
				for _, value := range taskMap {
					err := value.Run()
					if err != nil {
						allError = multierror.Append(allError, err)
					}
				}
				if allError != nil {
					return breverrors.WrapAndTrace(allError)
				}
			} else {
				if len(args) == 0 {
					return fmt.Errorf("provide a task name or --all")
				}
				if task, ok := taskMap[args[0]]; ok {
					err := task.Run()
					if err != nil {
						return breverrors.WrapAndTrace(err)
					}
				} else {
					fmt.Println("could not find task")
				}
			}
			return nil
		},
	}
	return cmd
}

func Tasks(_ *terminal.Terminal, _ TaskStore, _ TaskMap) error {
	return nil
}

func getTaskMap(store TaskStore) TaskMap {
	taskmap := make(TaskMap)
	vpnd := vpn.NewVPNDaemon(store)
	taskmap["vpnd"] = vpnd
	rpcd := server.NewRPCServerTask(store)
	taskmap["rpcd"] = rpcd
	sshcd := ssh.NewSSHConfigurerTask(store)
	taskmap["sshcd"] = sshcd
	return taskmap
}
