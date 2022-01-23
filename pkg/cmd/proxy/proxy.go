package proxy

import (
	"fmt"
	"regexp"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/huproxyclient"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/terminal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	allowedWorkspaceImage        = "brevdev/ubuntu-proxy:0.3.*"
	allowedWorkspaceInfraVersion = "v1.7.*"
)

type ProxyStore interface {
	GetAuthTokens() (*entity.AuthTokens, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	WritePrivateKey(pem string) error
	GetCurrentUserKeys() (*entity.UserKeys, error)
}

func NewCmdProxy(t *terminal.Terminal, store ProxyStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"hidden": ""},
		Use:                   "proxy",
		DisableFlagsInUseLine: true,
		Short:                 "http upgrade proxy",
		Long:                  "http upgrade proxy for ssh ProxyCommand directive to use",
		Args:                  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := Proxy(t, store, args[0])
			if err != nil {
				log.Error(err.Error())
			}
		},
	}

	return cmd
}

func Proxy(_ *terminal.Terminal, store ProxyStore, workspaceID string) error {
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

	url := makeProxyURL(workspace)
	err = huproxyclient.Run(url, store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func makeProxyURL(w *entity.Workspace) string {
	return fmt.Sprintf("wss://%s/proxy", w.GetSSHURL())
}

func CheckWorkspaceCanSSH(workspace *entity.Workspace) error {
	// todo greater than
	allowedInfra, err := regexp.Match(allowedWorkspaceInfraVersion, []byte(workspace.Version))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !allowedInfra {
		return fmt.Errorf("workspace of version %s is not supported with this cli version\n upgrade your workspace or downgrade your cli", workspace.Version)
	}
	allowedImage, err := regexp.Match(allowedWorkspaceImage, []byte(workspace.WorkspaceTemplate.Image))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !allowedImage {
		return fmt.Errorf("workspace image version %s is not supported with this cli version\n upgrade your workspace or downgrade your cli", workspace.WorkspaceTemplate.Image)
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
