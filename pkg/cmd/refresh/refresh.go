// Package refresh lists workspaces in the current org
package refresh

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"sync"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/cmdcontext"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/ssh"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

type RefreshStore interface {
	ssh.ConfigUpdaterStore
	ssh.SSHConfigurerV2Store
	GetCurrentUser() (*entity.User, error)
	GetCurrentUserKeys() (*entity.UserKeys, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetContextWorkspacesFor(orgID string, userID string) ([]entity.Workspace, error)
	GetAccessToken() (string, error)
	Chmod(string, fs.FileMode) error
	MkdirAll(string, fs.FileMode) error
	GetBrevCloudflaredBinaryPath() (string, error)
	Create(string) (io.WriteCloser, error)
}

func NewCmdRefresh(t *terminal.Terminal, store RefreshStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"configuration": ""},
		Use:         "refresh",
		Short:       "Force a refresh to the SSH config",
		Long:        "Force a refresh to the SSH config",
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
			t.Vprintf("%s", t.Green("brev has been refreshed\n"))
			return nil
		},
	}

	return cmd
}

func RunRefresh(store RefreshStore) error {
	cl := GetCloudflare(store)
	err := cl.DownloadCloudflaredBinaryIfItDNE()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	cu, err := GetConfigUpdater(store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = cu.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	privateKeyPath, err := store.GetPrivateKeyPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = store.Chmod(privateKeyPath, 0o600)
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

	wg.Go(func() {
		err := RunRefresh(rstore)
		if err != nil {
			res.er = err
		}
	})
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

	identity := resolveRefreshIdentity(store)

	cu := ssh.NewConfigUpdater(
		workspaceSSHStore{RefreshStore: store, org: identity.org, user: identity.user},
		configs,
		keys.PrivateKey,
	)
	cu.ExternalNodes = getExternalNodeSSHEntries(store, identity)

	return cu, nil
}

// refreshIdentity is the per-refresh identity
type refreshIdentity struct {
	org  *entity.Organization
	user *entity.User
}

// resolveRefreshIdentity best-effort resolves the org and user. A nil org or user just means the steps that need them are skipped.
func resolveRefreshIdentity(store RefreshStore) refreshIdentity {
	var identity refreshIdentity
	org, err := store.GetActiveOrganizationOrDefault()
	if err != nil {
		log.Printf("refresh: could not resolve active organization: %v", err)
	} else {
		identity.org = org
	}
	user, err := store.GetCurrentUser()
	if err != nil {
		log.Printf("refresh: could not resolve current user: %v", err)
	} else {
		identity.user = user
	}
	return identity
}

// getExternalNodeSSHEntries fetches external nodes and resolves their SSH details.
// This is best-effort: if anything fails, it returns nil so workspace SSH config is unaffected.
func getExternalNodeSSHEntries(store RefreshStore, identity refreshIdentity) []ssh.ExternalNodeSSHEntry {
	if identity.org == nil || identity.user == nil {
		return nil
	}
	org, user := identity.org, identity.user

	client := register.NewNodeServiceClient(store, config.GlobalConfig.GetBrevPublicAPIURL())
	resp, err := client.ListNodes(context.Background(), connect.NewRequest(&nodev1.ListNodesRequest{
		OrganizationId: org.ID,
		Options: &nodev1.ListNodesOptions{
			ExcludeConnectivityInfo: true,
		},
	}))
	if err != nil {
		return nil
	}

	var entries []ssh.ExternalNodeSSHEntry
	for _, node := range resp.Msg.GetItems() {
		entry := util.ResolveNodeSSHEntry(user.ID, node)
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	return entries
}

func GetCloudflare(refreshStore RefreshStore) store.Cloudflared {
	cl := store.NewCloudflare(refreshStore)
	return cl
}
