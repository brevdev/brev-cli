package workspacemanagerv2

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewWorkspaceManager(t *testing.T) {
	wm := NewWorkspaceManager(nil, nil)
	assert.NotNil(t, wm)
}

func Test_StartWorkspaceManager(t *testing.T) {
	ctx := context.Background()
	wm := NewWorkspaceManager(nil, nil)
	err := wm.Start(ctx, "")
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
	v := NewStaticVolume(",", "paht", strings.NewReader("boomm"))
	v = v.WithPathPrefix("prefix")
	assert.Equal(t, "prefix", v.FromMountPathPrefix)
}
