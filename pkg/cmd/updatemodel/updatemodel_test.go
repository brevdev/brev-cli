package updatemodel

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/go-git/go-git/v5"
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
