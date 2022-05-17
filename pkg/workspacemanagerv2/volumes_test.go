package workspacemanagerv2

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SymlinkVolume(t *testing.T) {
	err := os.MkdirAll("/tmp/test-symlink-volume/original", os.ModePerm)
	assert.Nil(t, err)
	_, err = os.OpenFile("/tmp/test-symlink-volume/original/file", os.O_CREATE, 0o600)
	assert.Nil(t, err)

	res := NewSymLinkVolume("/tmp/test-symlink-volume/original", "/tmp/test-symlink-volume/vol", "path")
	err = res.Setup(context.TODO())
	assert.Nil(t, err)
}
