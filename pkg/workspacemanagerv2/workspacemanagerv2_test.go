package workspacemanagerv2

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/stretchr/testify/assert"
)

type TestStore struct{}

var TestImage = "brevdev/ubuntu-proxy:0.3.17"

func (t TestStore) GetWorkspace(_ string) (*entity.Workspace, error) {
	return &entity.Workspace{
		WorkspaceTemplate: entity.WorkspaceTemplate{
			Image: TestImage,
		},
	}, nil
}

func (t TestStore) GetWorkspaceSetupParams(_ string) (*store.SetupParamsV0, error) {
	return &store.SetupParamsV0{
		WorkspaceHost:                    "",
		WorkspacePort:                    0,
		WorkspaceBaseRepo:                "",
		WorkspaceProjectRepo:             "",
		WorkspaceProjectRepoBranch:       "",
		WorkspaceApplicationStartScripts: []string{},
		WorkspaceUsername:                "",
		WorkspaceEmail:                   "",
		WorkspacePassword:                "",
		WorkspaceKeyPair: &store.KeyPair{
			PublicKeyData:  "",
			PrivateKeyData: "",
		},
		SetupScript:       new(string),
		ProjectFolderName: "",
		BrevPath:          "",
		DisableSetup:      false,
	}, nil
}

func (t TestStore) GetWorkspaceSecretsConfig(_ string) (interface{}, error) {
	return nil, nil
}

func (t TestStore) GetWorkspaceMeta(id string) (*store.WorkspaceMeta, error) {
	return &store.WorkspaceMeta{
		WorkspaceID:      id,
		WorkspaceGroupID: "",
		UserID:           "",
		OrganizationID:   "",
	}, nil
}

func Test_NewWorkspaceManager(t *testing.T) {
	cm := DockerContainerManager{}
	store := TestStore{}
	wm := NewWorkspaceManager(cm, store)
	assert.NotNil(t, wm)
}

func Test_StartWorkspaceManager(t *testing.T) {
	ctx := context.Background()
	cm := DockerContainerManager{}
	store := TestStore{}
	wm := NewWorkspaceManager(cm, store)
	err := wm.Start(ctx, "test")
	assert.Nil(t, err)
}

func Test_StopWorkspaceManager(t *testing.T) {
	ctx := context.Background()
	wm := NewWorkspaceManager(nil, nil)
	err := wm.Stop(ctx, "")
	assert.Nil(t, err)
}

func Test_ResetWorkspaceManager(t *testing.T) {
	ctx := context.Background()
	wm := NewWorkspaceManager(nil, nil)
	err := wm.Reset(ctx, "")
	assert.Nil(t, err)
}

func TestCreateWorkspace(t *testing.T) {
	v := NewStaticFiles("path", map[string]io.Reader{"doom": strings.NewReader("boomm")})
	v = v.WithPathPrefix("prefix")
	assert.Equal(t, "prefix", v.FromMountPathPrefix)
}
