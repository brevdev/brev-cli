package workspacemanager

import (
	"testing"
)

func Test_NewWorkspaceManager(t *testing.T) {
	NewWorkspaceManager()
	// assert.NotNil(t, wm)
}

func Test_StartWorkspaceManager(t *testing.T) {
	wm := NewWorkspaceManager()
	wm.Start("")
	// assert.Nil(t, _)
}

func Test_StopWorkspaceManager(t *testing.T) {
	wm := NewWorkspaceManager()
	wm.Stop("")
	// assert.Nil(t, _)
}

func Test_ResetWorkspaceManager(t *testing.T) {
	wm := NewWorkspaceManager()
	wm.Reset("")
	// assert.Nil(t, _)
}
