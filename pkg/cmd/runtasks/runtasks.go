package runtasks

import (
	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

func NewCmdRunTasks(t *terminal.Terminal) *cobra.Command {
	var detached bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "run-tasks",
		DisableFlagsInUseLine: true,
		Short:                 "manually run tasks",
		// Long:                  startLong,
		// Example:               startExample,
		Args: cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunTasks(t, detached)
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}
		},
	}
	cmd.Flags().BoolVarP(&detached, "detached", "d", false, "run the command in the background instead of blocking the shell")

	return cmd
}

func RunTasks(_ *terminal.Terminal, detached bool) error {
	ts := getDefaultTasks()
	if detached {
		err := tasks.RunTaskAsDaemon(ts, "") // TODO
		if err != nil {
			return errors.WrapAndTrace(err)
		}
	} else {
		err := tasks.RunTasks(ts)
		if err != nil {
			return errors.WrapAndTrace(err)
		}
	}
	return nil
}

func getDefaultTasks() []tasks.Task {
	return nil
}
