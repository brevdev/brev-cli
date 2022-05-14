package workspacemanagerv2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DockerGetContainer(t *testing.T) {
	dcm := DockerContainerManager{}
	res, err := dcm.GetContainer(context.TODO(), "dne")
	assert.Nil(t, err)
	assert.NotEmpty(t, res)
}
