package tasks

import (
	"github.com/brevdev/brev-cli/pkg/entity"
	rpcserver "github.com/brevdev/brev-cli/pkg/rpc"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/vpn"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type TaskMap map[string]tasks.Task

var all bool // used for configure command

// func init() {
// 	TaskMap := make()
// 	TaskMap["ssh"] = ssh.ConfigUpdater{
// 		Store: store,
// 		Configs: []ssh.Config{
// 			ssh.NewSSHConfigurerV2(
// 				store,
// 			),
// 		},
// 	}
// }

type TaskStore interface {
	CopyBin(targetBin string) error
	WriteString(path, data string) error
	RegisterNode(publicKey string) error
	GetOrCreateFile(path string) (afero.File, error)
	GetNetworkAuthKey() (*store.GetAuthKeyResponse, error)
	GetCurrentWorkspaceID() (string, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdTasks(t *terminal.Terminal, store TaskStore) *cobra.Command {
	taskMap := getTaskMap(store)
	cmd := &cobra.Command{
		Use:                   "tasks",
		DisableFlagsInUseLine: true,
		Short:                 "run background daemons for brev",
		Long:                  "run background daemons for brev",
		Run: func(cmd *cobra.Command, args []string) {
			err := Tasks(t, store, taskMap)
			if err != nil {
				log.Error(err)
			}
		},
	}

	configure := NewCmdConfigure(t, store, taskMap)
	cmd.AddCommand(configure)
	return cmd
}

func NewCmdConfigure(_ *terminal.Terminal, _ TaskStore, taskMap TaskMap) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure [task to configure]",
		Short: "configure system startup daemon for task",
		Long:  "configure system startup daemon for task",
		Run: func(cmd *cobra.Command, args []string) {
			if all {
				for _, value := range taskMap {
					err := value.Configure()
					if err != nil {
						log.Error(err)
					}
				}
			} else {
				if len(args) == 0 {
					log.Error("provide a task name or --all")
					return
				}
				if task, ok := taskMap[args[0]]; ok {
					err := task.Configure()
					if err != nil {
						log.Error(err)
					}
				}
			}
		},
	}
	cmd.PersistentFlags().BoolVarP(&all, "all", "a", false, "configure all tasks (must run this as root and pass --user")
	return cmd
}

func Tasks(_ *terminal.Terminal, _ TaskStore, _ TaskMap) error {
	return nil
}

func getTaskMap(store TaskStore) TaskMap {
	// cu := ssh.ConfigUpdater{
	// 	Store: store,
	// 	Configs: []ssh.Config{
	// 		ssh.NewSSHConfigurerV2(
	// 			store,
	// 		),
	// 	},
	// }

	taskmap := make(TaskMap)
	vpnd := vpn.NewVPNDaemon(store)
	taskmap["vpnd"] = vpnd
	rpcd := rpcserver.NewRPCServerTask(store)
	taskmap["rpcd"] = rpcd
	// taskmap["vpnd"] :=
	return taskmap
}
