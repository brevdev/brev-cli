package proxy

import (
	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/huproxyclient"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

func NewCmdProxy(t *terminal.Terminal) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "proxy",
		DisableFlagsInUseLine: true,
		Short:                 "http upgrade proxy",
		Long:                  "http upgrade proxy for ssh ProxyCommand directive to use",
		Args:                  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			err := Proxy(t, args[0], args[1])
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}
		},
	}

	return cmd
}

func Proxy(_ *terminal.Terminal, workspaceID string, url string) error {
	err := CheckWorkspaceCanSSH(workspaceID)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	err = huproxyclient.Run(url)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	return nil
}

func CheckWorkspaceCanSSH(workspaceID string) error {
	return nil
}
