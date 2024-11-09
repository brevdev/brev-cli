// Package refresh lists workspaces in the current org
package refresh

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"sync"

	"github.com/brevdev/brev-cli/pkg/cloudflared"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/ssh"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type RefreshStore interface {
	ssh.ConfigUpdaterStore
	ssh.SSHConfigurerV2Store
	GetCurrentUser() (*entity.User, error)
	GetCurrentUserKeys() (*entity.UserKeys, error)
	Chmod(string, fs.FileMode) error
	MkdirAll(string, fs.FileMode) error
	GetBrevCloudflaredBinaryPath() (string, error)
	Create(string) (io.WriteCloser, error)
}

func NewCmdRefresh(t *terminal.Terminal, store RefreshStore) *cobra.Command {
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
			err := RunRefresh(store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			t.Vprintf(t.Green("brev has been refreshed\n"))
			return nil
		},
	}

	return cmd
}

func RunRefresh(store RefreshStore) error {
	// TODO: use context
	ctx := context.TODO()
	if err := cloudflared.DownloadCloudflaredBinaryIfItDNE(ctx); err != nil {
		return fmt.Errorf("error downloading cloudflared binary: %w", err)
	}

	cu, err := GetConfigUpdater(store)
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

func RunRefreshAsync(rstore RefreshStore) *RefreshRes {
	var wg sync.WaitGroup

	res := RefreshRes{wg: &wg}

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := RunRefresh(rstore)
		if err != nil {
			res.er = err
		}
	}()
	return &res
}

func GetConfigUpdater(store RefreshStore) (*ssh.ConfigUpdater, error) {
	configs, err := ssh.GetSSHConfigs(store)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	keys, err := store.GetCurrentUserKeys()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	cu := ssh.NewConfigUpdater(store, configs, keys.PrivateKey)

	return cu, nil
}
