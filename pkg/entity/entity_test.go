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
