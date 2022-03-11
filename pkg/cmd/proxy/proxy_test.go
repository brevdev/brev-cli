package proxy

import (
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
)

func TestVersionParsing(t *testing.T) {
	_, err := version.NewVersion("1234")
	assert.NotNil(t, err)
}
