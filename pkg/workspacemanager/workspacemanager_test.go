package workspacemanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewWorkspaceManager(t *testing.T) {
	wm := NewWorkspaceManager()
	assert.NotNil(t, wm)
}

func Test_StartWorkspaceManager(t *testing.T) {
	wm := NewWorkspaceManager()
	err := wm.Start("")
	assert.Nil(t, err)
}

func Test_StopWorkspaceManager(t *testing.T) {
	wm := NewWorkspaceManager()
	err := wm.Stop("")
	assert.Nil(t, err)
}

func Test_ResetWorkspaceManager(t *testing.T) {
	wm := NewWorkspaceManager()
	err := wm.Reset("")
	assert.Nil(t, err)
}
