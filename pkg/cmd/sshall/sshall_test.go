package sshall

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/stretchr/testify/assert"
)

func Test_getActiveWorkspaces(t *testing.T) {
	_, err := getUserActiveWorkspaces()
	ae := brev_errors.ActiveOrgFileNotFound{}
	assert.ErrorIs(t, err, &ae)
}

func Test_getLocalPortForWorkspace(t *testing.T) {
	port := getRandomLocalPortForWorkspace("")
	assert.NotEmpty(t, port)
}

func Test_portforwardWorkspace(t *testing.T) {
	err := portforwardWorkspace("jtevxj5g5", "4444:4444")
	assert.Nil(t, err)
}

func Test_runSSHAll(t *testing.T) {
	workspaces, err := getUserActiveWorkspaces()
	assert.Nil(t, err)

	err = RunSSHAll(workspaces, getRandomLocalPortForWorkspace)
	assert.Nil(t, err)
}
