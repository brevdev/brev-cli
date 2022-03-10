package taskS

import (
	"os/user"

	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/brevdev/brev-cli/pkg/terminal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type TaskMap map[string]tasks.Task

var (
	all    bool   // used for configure command
	userID string // used for configure commmand
)

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

type TaskStore interface{}

func NewCmdTasks(t *terminal.Terminal, store TaskStore, taskMap TaskMap) *cobra.Command {
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

func NewCmdConfigure(t *terminal.Terminal, store TaskStore, taskMap TaskMap) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure [task to configure]",
		Short: "configure system startup daemon for task",
		Long:  "configure system startup daemon for task",
		Run: func(cmd *cobra.Command, args []string) {
			var userToConfigure *user.User
			var err error
			userToConfigure, err = user.Lookup(userID)
			if err != nil {
				_, ok := err.(*user.UnknownUserError)
				if !ok {
					userToConfigure, _ = user.LookupId(userID)
				}
			}
			if all {
				for _, value := range taskMap {
					err := value.Configure(userToConfigure)
					if err != nil {
						log.Error(err)
					}
				}
			}
		},
	}
	cmd.PersistentFlags().BoolVarP(&all, "all", "a", false, "configure all tasks (must run this as root and pass --user")
	cmd.PersistentFlags().StringVarP(&userID, "user", "u", "", "user id to configure tasks for (needed when run as root or with --all)")
	return cmd
}

func Tasks(_ *terminal.Terminal, _ TaskStore, taskMap TaskMap) error {
	return nil
}
