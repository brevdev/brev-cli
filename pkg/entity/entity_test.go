package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProjectFolderPathUsesSSHUser(t *testing.T) {
	workspace := Workspace{
		ID:      "workspace-1",
		SSHUser: "ubuntu",
		GitRepo: "https://github.com/brevdev/example.git",
	}

	projectFolderPath, err := workspace.GetProjectFolderPath()
	require.NoError(t, err)
	assert.Equal(t, "/home/ubuntu/example", projectFolderPath)
}

func TestWorkspaceRenameChangesLocalIdentifiersButPreservesProjectFolder(t *testing.T) {
	workspace := Workspace{
		ID:        "workspace-1",
		Name:      "old-instance-name",
		SSHUser:   "ubuntu",
		IDEConfig: IDEConfig{DefaultWorkingDir: "/mnt/persisted/old-instance-name"},
	}

	beforeProjectFolderPath, err := workspace.GetProjectFolderPath()
	require.NoError(t, err)
	assert.Equal(t, WorkspaceLocalID("old-instance-name"), workspace.GetLocalIdentifier())
	assert.Equal(t, WorkspaceLocalID("old-instance-name-host"), workspace.GetHostIdentifier())

	workspace.Name = "new-instance-name"

	afterProjectFolderPath, err := workspace.GetProjectFolderPath()
	require.NoError(t, err)
	assert.Equal(t, "workspace-1", workspace.ID)
	assert.Equal(t, WorkspaceLocalID("new-instance-name"), workspace.GetLocalIdentifier())
	assert.Equal(t, WorkspaceLocalID("new-instance-name-host"), workspace.GetHostIdentifier())
	assert.Equal(t, "/mnt/persisted/old-instance-name", beforeProjectFolderPath)
	assert.Equal(t, beforeProjectFolderPath, afterProjectFolderPath)
}
