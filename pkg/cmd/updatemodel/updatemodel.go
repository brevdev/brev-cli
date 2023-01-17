package updatemodel

import (
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/go-git/go-git/v5"
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
}

func NewCmdupdatemodel(t *terminal.Terminal, store updatemodelStore) *cobra.Command {
	var directory string
	cmd := &cobra.Command{
		Use:                   "updatemodel",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: updateModel{
			t:         t,
			store:     store,
			directory: directory,
			clone:     git.PlainClone,
			open:      func(path string) (repo, error) { return git.PlainOpen(path) },
		}.RunE,
	}
	cmd.Flags().StringVarP(&directory, "directory", "d", ".", "Directory to run command in")
	return cmd
}

type repo interface {
	Remotes() ([]*git.Remote, error)
}

type updateModel struct {
	t         *terminal.Terminal
	store     updatemodelStore
	directory string
	clone     func(path string, isBare bool, o *git.CloneOptions) (*git.Repository, error)
	open      func(path string) (repo, error)
}

func (u updateModel) RunE(_ *cobra.Command, _ []string) error {
	// this could be done in one go but this way is easier to reason about
	err := u.updateBE()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = u.updateENV()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (u updateModel) updateENV() error {
	remotes, err := u.remotes()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaceID, err := u.store.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspace, err := u.store.GetWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	reposv1FromBE := workspace.ReposV1
	reposv1FromENV := lo.Reduce(
		remotes,
		func(acc *entity.ReposV1, remote *git.Remote, _ int) *entity.ReposV1 {
			name := remote.Config().Name
			url := remote.Config().URLs[0]
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

	envLocalRepoMerger := &repoMerger{
		acc:   reposv1FromENV,
		repos: []*entity.ReposV1{reposv1FromBE},
	}

	errors := lo.Map(
		envLocalRepoMerger.ReposToClone(),
		func(repo *entity.RepoV1, _ int) error {
			_, err := u.clone(u.directory, false, &git.CloneOptions{
				URL: repo.GitRepo.Repository,
			})
			return err
		},
	)

	return breverrors.WrapAndTrace(
		lo.Reduce(
			errors,
			func(acc error, err error, _ int) error {
				if acc != nil && err != nil {
					return multierror.Append(acc, err)
				}
				if acc == nil && err != nil {
					return err
				}
				return acc
			},
			nil,
		),
	)
}

func (u updateModel) updateBE() error {
	remotes, err := u.remotes()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaceID, err := u.store.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspace, err := u.store.GetWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	reposv1FromBE := workspace.ReposV1
	reposv1FromENV := lo.Reduce(
		remotes,
		func(acc *entity.ReposV1, remote *git.Remote, _ int) *entity.ReposV1 {
			name := remote.Config().Name
			url := remote.Config().URLs[0]
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

	beLocalRepoMerger := &repoMerger{
		acc:   reposv1FromBE,
		repos: []*entity.ReposV1{reposv1FromENV},
	}

	_, err = u.store.ModifyWorkspace(
		workspaceID,
		&store.ModifyWorkspaceRequest{
			ReposV1: beLocalRepoMerger.MergeBE(),
		},
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
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
		func(repo *entity.RepoV1, _ int) bool {
			_, valueInENV := lo.Find(
				r.reposValues(),
				func(repo *entity.RepoV1) bool {
					return repo.GitRepo.Repository == repo.GitRepo.Repository
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
			values = append(values, &v)
		}
	}
	return values
}

func (r repoMerger) accValues() []*entity.RepoV1 {
	values := []*entity.RepoV1{}
	for _, v := range *r.acc {
		values = append(values, &v)
	}
	return values
}

func (u updateModel) remotes() ([]*git.Remote, error) {
	remotes := []*git.Remote{}
	err := filepath.Walk(u.directory, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			r, err := u.open(path)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			remotes, err := r.Remotes()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			remotes = append(remotes, remotes...)
		}
		return nil
	})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return remotes, nil
}
