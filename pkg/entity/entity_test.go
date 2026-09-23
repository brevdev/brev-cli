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

// Editor paths use saved configuration even when the display name changes.
func TestRenamedWorkspaceUsesStoredProjectFolder(t *testing.T) {
	workspace := Workspace{
		Name: "new-instance-name", SSHUser: "ubuntu",
		IDEConfig: IDEConfig{DefaultWorkingDir: "/mnt/persisted/old-instance-name"},
	}
	folder, err := workspace.GetProjectFolderPath()
	require.NoError(t, err)
	assert.Equal(t, "/mnt/persisted/old-instance-name", folder)
}
