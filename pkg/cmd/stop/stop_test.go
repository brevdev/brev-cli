package stop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStopWorkspaceSelf(t *testing.T) {
	err := stopThisWorkspace(nil, nil)
	assert.Nil(t, err)
}
