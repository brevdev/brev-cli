package sshall

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/stretchr/testify/assert"
)

func Test_getActiveWorkspaces(t *testing.T) {
	_, err := getActiveWorkspaces()
	ae := brev_errors.ActiveOrgFileNotFound{}
	assert.ErrorIs(t, err, &ae)
}

func Test_getLocalPortForWorkspace(t *testing.T) {
	port := getLocalPortForWorkspace("")
	assert.NotEmpty(t, port)
}
