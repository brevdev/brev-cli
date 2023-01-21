package updatemodel

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/parse/pkg/parse"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

type updatemodelStore interface {
	ModifyWorkspace(workspaceID string, options *store.ModifyWorkspaceRequest) (*entity.Workspace, error)
	GetCurrentWorkspaceID() (string, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	WriteString(path, data string) error
	UserHomeDir() (string, error)
	ListDirs(path string) ([]string, error)
	FileExists(filepath string) (bool, error)
	GetEnvSetupParams(wsid string) (*store.SetupParamsV0, error)
	GetCurrentUser() (*entity.User, error)
}

func NewCmdupdatemodel(t *terminal.Terminal, store updatemodelStore) *cobra.Command {
	var configure bool
	cmd := &cobra.Command{
		Use:                   "updatemodel",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: updateModel{
			t:     t,
			Store: store,
			clone: func(path string, isBare bool, o *git.CloneOptions) (*git.Repository, error) {
				workspaceID, err := store.GetCurrentWorkspaceID()
				if err != nil {
					return nil, breverrors.WrapAndTrace(err)
				}
				params, err := store.GetEnvSetupParams(workspaceID)
				if err != nil {
					return nil, breverrors.WrapAndTrace(err)
				}

				if params != nil && params.WorkspaceKeyPair != nil {
					keys := params.WorkspaceKeyPair
					pubkeys, err := ssh.NewPublicKeys("ubuntu", []byte(keys.PrivateKeyData), "")
					if err != nil {
						return nil, breverrors.WrapAndTrace(err)
					}
					o.Auth = pubkeys
				}

				r, err := git.PlainClone(path, isBare, o)
				return r, breverrors.WrapAndTrace(err)
			},
			open: func(path string) (repo, error) {
				r, err := git.PlainOpen(path)
				return r, breverrors.WrapAndTrace(err)
			},
			configure: configure,
		}.RunE,
	}

	cmd.Flags().BoolVarP(&configure, "configure", "c", false, "configure daemon")
	return cmd
}

type repo interface {
	Remotes() ([]*git.Remote, error)
}

type updateModel struct {
	t         *terminal.Terminal
	Store     updatemodelStore
	clone     func(path string, isBare bool, o *git.CloneOptions) (*git.Repository, error)
	open      func(path string) (repo, error)
	configure bool
}

func (u updateModel) RunE(_ *cobra.Command, _ []string) error {
	if u.configure {
		return breverrors.WrapAndTrace(
			DaemonConfigurer{
				Store: u.Store,
			}.Configure(),
		)
	}

	remotes, err := u.remotes()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaceID, err := u.Store.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspace, err := u.Store.GetWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// there should only be one but sometimes there are more and thats not
	// handled right now
	urls := lo.FlatMap(
		remotes,
		func(remote *git.Remote, _ int) []string {
			return remote.Config().URLs
		},
	)

	// filter user dotbrev remote url
	user, err := u.Store.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	dotbrev := user.BaseWorkspaceRepo
	urls = lo.Filter(
		urls,
		func(url string, _ int) bool {
			return !parse.CMP(url, dotbrev)
		},
	)

	reposv1FromENV := makeReposFromRemotes(urls)

	// merge reposv0 and gitrepo field into reposv1fromBE

	reposv1FromBE := make(entity.ReposV1)
	reposv1FromBE[entity.RepoName(parse.GetRepoNameFromOrigin(workspace.GitRepo))] = entity.RepoV1{
		GitRepo: entity.GitRepo{
			Repository: workspace.GitRepo,
		},
		Type: entity.GitRepoType,
	}

	for key, val := range workspace.ReposV0 {
		reposv1FromBE[key] = entity.RepoV1{
			GitRepo: entity.GitRepo{
				Repository: val.Repository,
			},
			Type: entity.GitRepoType,
		}
	}
	if workspace.ReposV1 != nil {
		for key, val := range *workspace.ReposV1 {
			key := key
			val := val
			reposv1FromBE[key] = val
		}
	}

	rm := &repoMerger{
		acc:   &reposv1FromBE,
		repos: []*entity.ReposV1{reposv1FromENV},
	}
	_, err = u.Store.ModifyWorkspace(
		workspaceID,
		&store.ModifyWorkspaceRequest{
			ReposV1: rm.MergeBE(),
		},
	)

	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	dir, err := u.Store.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	cloneErrors := lo.Map(
		rm.ReposToClone(),
		func(repo *entity.RepoV1, _ int) error {
			_, err := u.clone(dir, false, &git.CloneOptions{
				URL: repo.GitRepo.Repository,
			})
			return breverrors.WrapAndTrace(err)
		},
	)
	return breverrors.WrapAndTrace(
		lo.Reduce(
			cloneErrors,
			func(acc error, err error, _ int) error {
				if err != nil {
					txt := fmt.Sprintf("%s", err)
					if strings.Contains(txt, "already exists") {
						return acc
					}
					if acc == nil {
						return breverrors.WrapAndTrace(err)
					}
					return multierror.Append(acc, err)
				}
				if acc == nil && err != nil {
					return breverrors.WrapAndTrace(err)
				}
				return acc
			},
			nil,
		),
	)
}

type stringWriter interface {
	WriteString(path, data string) error
}

type DaemonConfigurer struct {
	Store stringWriter
}

func (dc DaemonConfigurer) Configure() error {
	// create systemd service file to run
	// brev updatemodel -d /home/ubuntu
	configFile := filepath.Join("/etc/systemd/system", "brev-updatemodel.service")
	err := dc.Store.WriteString(
		configFile,
		`[Unit]
Description=Brev Update Model
After=network.target

[Service]
Type=simple
User=ubuntu
ExecStart=/usr/bin/brev updatemodel -d /home/ubuntu
`)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if autostartconf.ShouldSymlink() {
		symlinkTarget := path.Join("/etc/systemd/system/default.target.wants/", "brev-updatemodel.service")
		err2 := os.Symlink(configFile, symlinkTarget)
		if err2 != nil {
			return breverrors.WrapAndTrace(err2)
		}
	}
	// create systemd timer to run every 5 seconds
	err = dc.Store.WriteString(
		"/etc/systemd/system/brev-updatemodel.timer",
		`[Unit]
Description=Brev Update Model Timer

[Timer]
OnBootSec=5
OnUnitActiveSec=5

[Install]
WantedBy=timers.target
`)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// enable timer
	err = autostartconf.ExecCommands(
		[][]string{
			{"systemctl", "enable", "brev-updatemodel.timer"},
			{"systemctl", "start", "brev-updatemodel.timer"},
			{"systemctl", "daemon-reload"},
		},
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func makeReposFromRemotes(remotes []string) *entity.ReposV1 {
	return lo.Reduce(
		remotes,
		func(acc *entity.ReposV1, remote string, _ int) *entity.ReposV1 {
			name := parse.GetRepoNameFromOrigin(remote)
			url := parse.GetSSHURLFromOrigin(remote)
			a := *acc
			a[entity.RepoName(name)] = entity.RepoV1{
				Type: entity.GitRepoType,
				GitRepo: entity.GitRepo{
					Repository: url,
				},
			}
			return &a
		},
		&entity.ReposV1{},
	)
}

type repoMerger struct {
	acc   *entity.ReposV1
	repos []*entity.ReposV1
}

func (r *repoMerger) MergeBE() *entity.ReposV1 {
	for _, repo := range r.repos {
		for k, v := range *repo {
			if _, ok := (*r.acc)[k]; ok {
				continue
			}
			_, valueInAcc := lo.Find(
				r.accValues(),
				func(repo *entity.RepoV1) bool {
					return repo.GitRepo.Repository == v.GitRepo.Repository
				},
			)
			if valueInAcc {
				continue
			}
			(*r.acc)[k] = v
		}
	}
	return r.acc
}

func (r *repoMerger) ReposToClone() []*entity.RepoV1 {
	// repos present in the BE but not in the ENV
	return lo.Filter(
		r.accValues(),
		func(accrepo *entity.RepoV1, _ int) bool {
			_, valueInENV := lo.Find(
				r.reposValues(),
				func(repo *entity.RepoV1) bool {
					return accrepo.GitRepo.Repository == repo.GitRepo.Repository
				},
			)
			return !valueInENV
		},
	)
}

func (r repoMerger) reposValues() []*entity.RepoV1 {
	values := []*entity.RepoV1{}
	for _, repo := range r.repos {
		for _, v := range *repo {
			// explicit memory aliasing in for loop.
			v := v
			values = append(values, &v)
		}
	}
	return values
}

func (r repoMerger) accValues() []*entity.RepoV1 {
	if r.acc == nil {
		return []*entity.RepoV1{}
	}
	values := []*entity.RepoV1{}
	for _, v := range *r.acc {
		newV := v
		values = append(values, &newV)
	}
	return values
}

func (u updateModel) remotes() ([]*git.Remote, error) {
	dir, err := u.Store.UserHomeDir()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	dirs, err := u.Store.ListDirs(dir)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// filter dirs that start with .
	dirs = lo.Filter(
		dirs,
		func(dir string, _ int) bool {
			return !strings.HasPrefix(dir, ".")
		},
	)

	dirsWithGit := []string{}
	for _, dir := range dirs {
		gitDir := path.Join(dir, ".git")
		exists, err := u.Store.FileExists(gitDir)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if exists {
			dirsWithGit = append(dirsWithGit, dir)
		}
	}

	repos := []*git.Repository{}
	for _, dir := range dirsWithGit {
		repo, err := git.PlainOpen(dir)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		repos = append(repos, repo)
	}

	remotes := []*git.Remote{}
	for _, repo := range repos {
		repoRemotes, err := repo.Remotes()
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		remotes = append(remotes, repoRemotes...)
	}
	return remotes, nil
}
