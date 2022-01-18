package proxy

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/huproxyclient"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

const (
	workspaceTemplateVersion = "placeholder"
	workspaceInfraVersion    = "placeholder"
)

type ProxyStore interface {
	GetAuthTokens() (*entity.AuthTokens, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	WritePrivateKey(pem string) error
	GetCurrentUserKeys() (*entity.UserKeys, error)
}

func NewCmdProxy(t *terminal.Terminal, store ProxyStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "proxy",
		DisableFlagsInUseLine: true,
		Short:                 "http upgrade proxy",
		Long:                  "http upgrade proxy for ssh ProxyCommand directive to use",
		Args:                  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			err := Proxy(t, store, args[0], args[1])
			if err != nil {
				t.Vprint(t.Red(err.Error()))
			}
		},
	}

	return cmd
}

func Proxy(_ *terminal.Terminal, store ProxyStore, workspaceID string, url string) error {
	workspace, err := store.GetWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = CheckWorkspaceCanSSH(workspace)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = WriteUserPrivateKey(store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = huproxyclient.Run(url, store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func CheckWorkspaceCanSSH(workspace *entity.Workspace) error {
	// todo greater than
	if workspaceInfraVersion != workspace.Version {
		return fmt.Errorf("invalid workspace worksapce version %s", workspace.Version)
	}
	if workspaceTemplateVersion != workspace.WorkspaceTemplateID {
		return fmt.Errorf("invalid workspace worksapce Template id %s", workspace.WorkspaceTemplateID)
	}
	if workspace.Status != "RUNNING" {
		return fmt.Errorf("workspace is not in RUNNING state, status: %s", workspace.Status)
	}
	return nil
}

func WriteUserPrivateKey(store ProxyStore) error {
	workspaceGroupClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(
		store,
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	privateKey := workspaceGroupClientMapper.GetPrivateKey()
	err = store.WritePrivateKey(privateKey)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
