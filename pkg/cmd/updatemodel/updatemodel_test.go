package updatemodel

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/go-git/go-git/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
)

type updatemodelStoreMock struct{}

func (u updatemodelStoreMock) ModifyWorkspace(workspaceID string, options *store.ModifyWorkspaceRequest) (*entity.Workspace, error) {
	return nil, nil
}

func (u updatemodelStoreMock) GetCurrentWorkspaceID() (string, error) {
	return "test", nil
}

func (u updatemodelStoreMock) GetWorkspace(workspaceID string) (*entity.Workspace, error) {
	reposv1 := entity.ReposV1{
		entity.RepoName("test"): {
			Type: entity.GitRepoType,
			GitRepo: entity.GitRepo{
				Repository: "foo",
			},
		},
	}
	return &entity.Workspace{
		ReposV1: &reposv1,
	}, nil
}

func (u updatemodelStoreMock) WriteString(path, data string) error {
	return nil
}

func mockPlainClone(path string, isBare bool, o *git.CloneOptions) (*git.Repository, error) {
	return nil, nil
}

type remotes struct{}

func (r remotes) Remotes() ([]*git.Remote, error) {
	return nil, nil
}

func mockPlainOpen(path string) (repo, error) {
	return remotes{}, nil
}

func TestUpdateModel_RunE(t *testing.T) {
	type fields struct {
		t         *terminal.Terminal
		Store     updatemodelStore
		directory string
		clone     func(path string, isBare bool, o *git.CloneOptions) (*git.Repository, error)
		open      func(path string) (repo, error)
		configure bool
	}
	type args struct {
		in0 *cobra.Command
		in1 []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		// TODO: Add test cases.
		{
			name: "test",
			fields: fields{
				t:         nil,
				Store:     updatemodelStoreMock{},
				directory: ".",
				clone:     mockPlainClone,
				open:      mockPlainOpen,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := UpdateModel{
				t:         tt.fields.t,
				Store:     tt.fields.Store,
				directory: tt.fields.directory,
				clone:     tt.fields.clone,
				open:      tt.fields.open,
				configure: tt.fields.configure,
			}
			if err := u.RunE(tt.args.in0, tt.args.in1); (err != nil) != tt.wantErr {
				t.Errorf("UpdateModel.RunE() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_makeReposFromRemotes(t *testing.T) {
	type args struct {
		remotes []string
	}
	tests := []struct {
		name string
		args args
		want *entity.ReposV1
	}{
		// TODO: Add test cases.
		{
			name: "test",
			args: args{
				remotes: []string{"git@github.com:brevdev/brev-cli.git", "git@github.com:brevdev/brev-deploy.git"},
			},
			want: &entity.ReposV1{
				entity.RepoName("brev-cli"): {
					Type: entity.GitRepoType,
					GitRepo: entity.GitRepo{
						Repository: "git@github.com:brevdev/brev-cli.git",
					},
				},
				entity.RepoName("brev-deploy"): {
					Type: entity.GitRepoType,
					GitRepo: entity.GitRepo{
						Repository: "git@github.com:brevdev/brev-deploy.git",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeReposFromRemotes(tt.args.remotes)
			// cmp.Diff is used to compare the two structs
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("makeReposFromRemotes() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
