// Package refresh lists workspaces in the current org
package refresh

import (
	"fmt"
	"sync"

	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/k8s"
	"github.com/brevdev/brev-cli/pkg/ssh"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type RefreshStore interface {
	ssh.ConfigUpdaterStore
	ssh.SSHConfigurerV2Store
	GetCurrentUser() (*entity.User, error)
	k8s.K8sStore
}

func NewCmdRefresh(t *terminal.Terminal, store RefreshStore) *cobra.Command {
	var runRemoteCMD bool
	cmd := &cobra.Command{
		Annotations: map[string]string{"housekeeping": ""},
		Use:         "refresh",
		Short:       "Force a refresh to the ssh config",
		Long:        "Force a refresh to the ssh config",
		Example:     `brev refresh`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("refreshing brev...")
			err := RunRefresh(store, runRemoteCMD)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			t.Vprintf(t.Green("brev has been refreshed\n"))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&runRemoteCMD, "remote", "r", true, "run remote commands")

	return cmd
}

func RunRefresh(store RefreshStore, runRemoteCMD bool) error {
	cu, err := GetConfigUpdater(store, runRemoteCMD)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = cu.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

type RefreshRes struct {
	wg *sync.WaitGroup
	er error
}

func (r *RefreshRes) Await() error {
	r.wg.Wait()
	return r.er
}

func RunRefreshAsync(rstore RefreshStore, runRemoteCMD bool) *RefreshRes {
	var wg sync.WaitGroup

	res := RefreshRes{wg: &wg}

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := RunRefresh(rstore, runRemoteCMD)
		if err != nil {
			res.er = err
		}
	}()
	return &res
}

func GetConfigUpdater(store RefreshStore, runRemotCMD bool) (*ssh.ConfigUpdater, error) {
	configs, err := ssh.GetSSHConfigs(store, runRemotCMD)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// get private key and set here
	workspaceGroupClientMapper, err := k8s.NewDefaultWorkspaceGroupClientMapper(store) // to resolve
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	privateKey := workspaceGroupClientMapper.GetPrivateKey()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	cu := ssh.NewConfigUpdater(store, configs, privateKey)

	return cu, nil
}
